package sqlite

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	sqlite3 "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// An encrypted database is the whole SQLite database sealed with AES-256-GCM
// under a key derived from its password (PBKDF2-SHA256, a random salt per
// file). While the app has it open it lives only in memory: nothing
// unencrypted is written to disk. Changes are sealed and saved shortly after
// they're made, and when the database is closed.
//
// The file: magic, PBKDF2 iterations (uint32), salt, GCM nonce, then the
// sealed database; everything before it is authenticated with it, so a wrong
// password and a damaged file both fail to open.

var (
	// ErrEncrypted is returned for opening an encrypted database without its
	// password.
	ErrEncrypted = errors.New("the database is encrypted: it needs its password")
	// ErrWrongPassword is returned when a password doesn't open a database.
	ErrWrongPassword = errors.New("wrong password, or the file is damaged")
)

const (
	vaultMagic = "cents-encrypted-database-v1\n"
	saltSize   = 16
	nonceSize  = 12
	keySize    = 32
	// MinPasswordLength is the shortest password accepted, in characters.
	MinPasswordLength = 8
)

// kdfIterations is how hard deriving a key is; kept with each file, so it
// can grow later. Tests lower it.
var kdfIterations = 600_000

// saveDelay is how soon after a change it's saved.
var saveDelay = 400 * time.Millisecond

// IsEncrypted reports whether the file at path is an encrypted database.
func IsEncrypted(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	head := make([]byte, len(vaultMagic))
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, err
	}

	return n == len(vaultMagic) && string(head) == vaultMagic, nil
}

// CheckPassword checks a new password is long enough.
func CheckPassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return fmt.Errorf("the password needs at least %d characters", MinPasswordLength)
	}

	return nil
}

type vaultKey struct {
	key        []byte
	salt       []byte
	iterations int
}

func newKey(password string) (vaultKey, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return vaultKey{}, err
	}

	return deriveKey(password, salt, kdfIterations)
}

func deriveKey(password string, salt []byte, iterations int) (vaultKey, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keySize)
	if err != nil {
		return vaultKey{}, err
	}

	return vaultKey{key: key, salt: salt, iterations: iterations}, nil
}

func (k vaultKey) header() []byte {
	h := make([]byte, 0, len(vaultMagic)+4+saltSize)
	h = append(h, vaultMagic...)
	h = binary.BigEndian.AppendUint32(h, uint32(k.iterations))
	return append(h, k.salt...)
}

func (k vaultKey) seal(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(k.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	header := k.header()
	out := append(append([]byte{}, header...), nonce...)
	return gcm.Seal(out, nonce, plain, header), nil
}

// unseal opens a sealed database with password, returning it and its key.
func unseal(data []byte, password string) ([]byte, vaultKey, error) {
	headerSize := len(vaultMagic) + 4 + saltSize
	if len(data) < headerSize+nonceSize || !bytes.HasPrefix(data, []byte(vaultMagic)) {
		return nil, vaultKey{}, errors.New("not an encrypted cents database")
	}

	iterations := int(binary.BigEndian.Uint32(data[len(vaultMagic):]))
	if iterations < 1 || iterations > 100_000_000 {
		return nil, vaultKey{}, ErrWrongPassword
	}

	salt := data[len(vaultMagic)+4 : headerSize]
	key, err := deriveKey(password, append([]byte{}, salt...), iterations)
	if err != nil {
		return nil, vaultKey{}, err
	}

	block, err := aes.NewCipher(key.key)
	if err != nil {
		return nil, vaultKey{}, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, vaultKey{}, err
	}

	nonce := data[headerSize : headerSize+nonceSize]
	plain, err := gcm.Open(nil, nonce, data[headerSize+nonceSize:], data[:headerSize])
	if err != nil {
		return nil, vaultKey{}, ErrWrongPassword
	}

	return plain, key, nil
}

// writeFile replaces path with data: written next to it first, then moved
// over, so a failed save leaves the old file.
func writeFile(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".saving-*")
	if err != nil {
		return err
	}

	name := temp.Name()
	_, err = temp.Write(data)
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(name, 0o600)
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
	}

	return err
}

// --- the database in memory --------------------------------------------------

// openMemory opens a database held in memory, loaded from plain (a whole
// SQLite database) unless it's empty. One connection only: each would have
// its own memory otherwise.
func openMemory(plain []byte) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(0)

	if len(plain) > 0 {
		if err := withConn(db, func(c *sqlite3.SQLiteConn) error { return c.Deserialize(plain, "main") }); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("failed to load the database: %w", err)
		}
	}

	return db, nil
}

// withConn runs fn on db's SQLite connection.
func withConn(db *gorm.DB, fn func(*sqlite3.SQLiteConn) error) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		c, ok := driverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("not a SQLite connection")
		}

		return fn(c)
	})
}

// serialize is the whole database as a SQLite file's bytes.
func serialize(db *gorm.DB) ([]byte, error) {
	var plain []byte
	err := withConn(db, func(c *sqlite3.SQLiteConn) error {
		var err error
		plain, err = c.Serialize("main")
		return err
	})

	return plain, err
}

// --- keeping it saved --------------------------------------------------------

// Vault keeps an encrypted database's file up to date with the database in
// memory.
type Vault struct {
	path string
	db   *gorm.DB

	mu  sync.Mutex
	key vaultKey

	dirty   atomic.Bool
	wake    chan struct{}
	stop    chan struct{}
	stopped chan struct{}

	// failed is the last save's error, nil once a save works again.
	errMu  sync.Mutex
	failed error
}

func (v *Vault) setFailed(err error) {
	v.errMu.Lock()
	v.failed = err
	v.errMu.Unlock()
}

// Failed is the last save's error, nil when the file is up to date or
// being saved.
func (v *Vault) Failed() error {
	v.errMu.Lock()
	defer v.errMu.Unlock()
	return v.failed
}

func newVault(path string, db *gorm.DB, key vaultKey) (*Vault, error) {
	v := &Vault{path: path, db: db, key: key, wake: make(chan struct{}, 1), stop: make(chan struct{}), stopped: make(chan struct{})}

	// Every change marks the database to be saved.
	mark := func(tx *gorm.DB) {
		if tx.Error == nil {
			v.changed()
		}
	}
	for _, register := range []func() error{
		func() error { return db.Callback().Create().After("gorm:create").Register("cents:vault", mark) },
		func() error { return db.Callback().Update().After("gorm:update").Register("cents:vault", mark) },
		func() error { return db.Callback().Delete().After("gorm:delete").Register("cents:vault", mark) },
		func() error { return db.Callback().Raw().After("gorm:raw").Register("cents:vault", mark) },
	} {
		if err := register(); err != nil {
			return nil, err
		}
	}

	go v.run()
	return v, nil
}

func (v *Vault) changed() {
	v.dirty.Store(true)
	select {
	case v.wake <- struct{}{}:
	default:
	}
}

func (v *Vault) run() {
	defer close(v.stopped)
	for {
		select {
		case <-v.stop:
			return
		case <-v.wake:
		}

		// Changes come in bursts; save once they settle.
		select {
		case <-v.stop:
			return
		case <-time.After(saveDelay):
		}

		if v.dirty.Swap(false) {
			if err := v.Save(); err != nil {
				v.setFailed(err)
				v.changed()
			}
		}
	}
}

// Save seals the database and writes its file now.
func (v *Vault) Save() error {
	plain, err := serialize(v.db)
	if err != nil {
		return fmt.Errorf("failed to read the database to save it: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	sealed, err := v.key.seal(plain)
	if err != nil {
		return err
	}

	if err := writeFile(v.path, sealed); err != nil {
		return fmt.Errorf("failed to save the encrypted database: %w", err)
	}

	v.setFailed(nil)
	return nil
}

// SaveTo writes the database, sealed with the same password, to another
// file (a backup).
func (v *Vault) SaveTo(path string) error {
	plain, err := serialize(v.db)
	if err != nil {
		return err
	}

	v.mu.Lock()
	sealed, err := v.key.seal(plain)
	v.mu.Unlock()
	if err != nil {
		return err
	}

	return writeFile(path, sealed)
}

// ChangePassword seals the database under a new password from now on.
func (v *Vault) ChangePassword(password string) error {
	key, err := newKey(password)
	if err != nil {
		return err
	}

	v.mu.Lock()
	old := v.key
	v.key = key
	v.mu.Unlock()

	if err := v.Save(); err != nil {
		v.mu.Lock()
		v.key = old
		v.mu.Unlock()
		return err
	}

	return nil
}

// Close stops saving in the background and saves what's left.
func (v *Vault) Close() error {
	select {
	case <-v.stop:
		return nil
	default:
	}

	close(v.stop)
	<-v.stopped

	if v.dirty.Swap(false) {
		return v.Save()
	}

	if v.Failed() != nil {
		return v.Save()
	}

	return nil
}

// Path is the encrypted file.
func (v *Vault) Path() string {
	return v.path
}
