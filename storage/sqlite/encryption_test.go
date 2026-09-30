package sqlite

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
)

func init() {
	// Fast for tests; files keep their own count, so this changes nothing
	// for opening them.
	kdfIterations = 1000
	saveDelay = 20 * time.Millisecond
}

const marker = "hidden-marker-7f3a"

func comments(t *testing.T, s *SQLiteStorage) map[string]bool {
	t.Helper()
	entries, err := s.LoadCashflows()
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, e := range entries {
		got[e.Comment] = true
	}
	return got
}

func TestSealing(t *testing.T) {
	key, err := newKey("correct horse")
	if err != nil {
		t.Fatal(err)
	}

	sealed, err := key.seal([]byte("plain " + marker))
	if err != nil || bytes.Contains(sealed, []byte(marker)) {
		t.Fatalf("expected the text hidden: %v", err)
	}

	if plain, _, err := unseal(sealed, "correct horse"); err != nil || string(plain) != "plain "+marker {
		t.Fatalf("unexpected %q %v", plain, err)
	}

	if _, _, err := unseal(sealed, "wrong horse"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected a wrong password: %v", err)
	}

	tampered := append([]byte{}, sealed...)
	tampered[len(tampered)-1] ^= 1
	if _, _, err := unseal(tampered, "correct horse"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected a damaged file to fail: %v", err)
	}

	// The header is checked too: a changed salt fails.
	tampered = append([]byte{}, sealed...)
	tampered[len(vaultMagic)+5] ^= 1
	if _, _, err := unseal(tampered, "correct horse"); err == nil {
		t.Fatal("expected a changed header to fail")
	}

	if err := CheckPassword("short"); err == nil {
		t.Fatal("expected a short password refused")
	}
}

func TestEncryptedDatabase(t *testing.T) {
	s, path := openTest(t)
	if err := s.SaveSettingExpenseCategory(&settings.SettingExpenseCategory{CategoryName: "Food"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCashflow(&cashflow.CashflowEntry{Category: "Food", Comment: marker, EntryDate: time.Now()}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Encrypt(path, "short"); err == nil {
		t.Fatal("expected a short password refused")
	}

	s, err := s.Encrypt(path, "correct horse")
	if err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte(marker)) || bytes.Contains(raw, []byte("SQLite format")) {
		t.Fatal("expected nothing readable in the file")
	}
	if err := CheckDatabase(path); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("expected CheckDatabase to say encrypted: %v", err)
	}
	if _, _, err := OpenDatabase(path); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("expected OpenDatabase to say encrypted: %v", err)
	}
	if !s.Encrypted() || !comments(t, s)[marker] {
		t.Fatal("expected the data still there, encrypted")
	}

	// A change is saved shortly, without closing.
	if err := s.CreateCashflow(&cashflow.CashflowEntry{Category: "Food", Comment: "later", EntryDate: time.Now()}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * saveDelay)
	db, vault, err := OpenEncrypted(path, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if got := comments(t, NewEncryptedStorage(db, vault)); !got["later"] || !got[marker] {
		t.Fatalf("expected the change saved: %v", got)
	}
	_ = NewEncryptedStorage(db, vault).Close()

	if _, _, err := OpenEncrypted(path, "wrong horse"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected a wrong password: %v", err)
	}

	// Password change: the old one stops working.
	if err := s.ChangePassword("wrong horse", "battery staple"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected the current password checked: %v", err)
	}
	if err := s.ChangePassword("correct horse", "battery staple"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenEncrypted(path, "correct horse"); err == nil {
		t.Fatal("expected the old password to fail")
	}

	// A backup is encrypted with the same password.
	backup := filepath.Join(filepath.Dir(path), "backup.db")
	if err := s.Backup(backup); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(backup); bytes.Contains(raw, []byte(marker)) {
		t.Fatal("expected the backup encrypted")
	}
	if err := s.Backup(path); err == nil {
		t.Fatal("expected backing up onto the database itself to fail")
	}
	bdb, bvault, err := OpenEncrypted(backup, "battery staple")
	if err != nil {
		t.Fatal(err)
	}
	_ = NewEncryptedStorage(bdb, bvault).Close()

	// Closing saves what's left.
	if err := s.CreateCashflow(&cashflow.CashflowEntry{Category: "Food", Comment: "at close", EntryDate: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db, vault, err = OpenEncrypted(path, "battery staple")
	if err != nil {
		t.Fatal(err)
	}
	s = NewEncryptedStorage(db, vault)
	if !comments(t, s)["at close"] {
		t.Fatal("expected the last change saved at close")
	}

	// Decrypt: a plain database again, everything in it.
	if _, err := s.Decrypt("wrong horse"); err == nil {
		t.Fatal("expected the current password checked")
	}
	plain, err := s.Decrypt("battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if plain.Encrypted() || len(comments(t, plain)) != 3 {
		t.Fatalf("expected all three entries, plain: %v", comments(t, plain))
	}
	if err := CheckDatabase(path); err != nil {
		t.Fatalf("expected a plain database: %v", err)
	}
}
