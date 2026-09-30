package sqlite

import (
	"errors"
	"fmt"
	"os"

	"gorm.io/gorm"
)

// OpenEncrypted opens the encrypted database at path with its password,
// into memory (see vault.go).
func OpenEncrypted(path, password string) (*gorm.DB, *Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read the database: %w", err)
	}

	plain, key, err := unseal(data, password)
	if err != nil {
		return nil, nil, err
	}

	db, err := openMemory(plain)
	if err != nil {
		return nil, nil, err
	}

	if err := prepare(db, ""); err != nil {
		closeDB(db)
		return nil, nil, err
	}

	vault, err := newVault(path, db, key)
	if err != nil {
		closeDB(db)
		return nil, nil, err
	}

	// Preparing may have changed something (today's rates, say).
	if err := vault.Save(); err != nil {
		_ = vault.Close()
		closeDB(db)
		return nil, nil, err
	}

	return db, vault, nil
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// Encrypted reports whether the database is encrypted.
func (s *SQLiteStorage) Encrypted() bool {
	return s.vault != nil
}

// Encrypt turns this plain database, the file at path, into an encrypted
// one with password. It returns the storage to use from now on; this one
// is closed. The plain file is replaced; copies made of it before (backups)
// stay as they are.
func (s *SQLiteStorage) Encrypt(path, password string) (*SQLiteStorage, error) {
	if s.vault != nil {
		return nil, errors.New("the database is encrypted already; change its password instead")
	}

	if err := CheckPassword(password); err != nil {
		return nil, err
	}

	plain, err := serialize(s.db)
	if err != nil {
		return nil, fmt.Errorf("failed to read the database: %w", err)
	}

	key, err := newKey(password)
	if err != nil {
		return nil, err
	}

	sealed, err := key.seal(plain)
	if err != nil {
		return nil, err
	}

	// Ready in memory before the file changes, so a failure leaves it be.
	mem, err := openMemory(plain)
	if err != nil {
		return nil, err
	}

	if err := prepare(mem, ""); err != nil {
		closeDB(mem)
		return nil, err
	}

	if err := writeFile(path, sealed); err != nil {
		closeDB(mem)
		return nil, fmt.Errorf("failed to save the encrypted database: %w", err)
	}

	closeDB(s.db)
	for _, extra := range []string{"-journal", "-wal", "-shm"} {
		_ = os.Remove(path + extra)
	}

	vault, err := newVault(path, mem, key)
	if err != nil {
		return nil, err
	}

	return NewEncryptedStorage(mem, vault), nil
}

// checkCurrent makes sure password is the database's.
func (s *SQLiteStorage) checkCurrent(password string) error {
	if s.vault == nil {
		return errors.New("the database isn't encrypted")
	}

	data, err := os.ReadFile(s.vault.Path())
	if err != nil {
		return err
	}

	_, _, err = unseal(data, password)
	return err
}

// ChangePassword seals the database under next from now on, once current is
// its password.
func (s *SQLiteStorage) ChangePassword(current, next string) error {
	if err := s.checkCurrent(current); err != nil {
		return err
	}

	if err := CheckPassword(next); err != nil {
		return err
	}

	return s.vault.ChangePassword(next)
}

// Decrypt turns this encrypted database back into a plain file, once
// current is its password. It returns the storage to use from now on; this
// one is closed.
func (s *SQLiteStorage) Decrypt(current string) (*SQLiteStorage, error) {
	if err := s.checkCurrent(current); err != nil {
		return nil, err
	}

	path := s.vault.Path()
	plain, err := serialize(s.db)
	if err != nil {
		return nil, fmt.Errorf("failed to read the database: %w", err)
	}

	if err := s.vault.Close(); err != nil {
		return nil, err
	}

	if err := writeFile(path, plain); err != nil {
		return nil, fmt.Errorf("failed to save the database: %w", err)
	}

	closeDB(s.db)
	db, _, err := OpenDatabase(path)
	if err != nil {
		return nil, err
	}

	return NewSQLiteStorage(db), nil
}
