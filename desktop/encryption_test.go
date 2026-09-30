package desktop

import (
	"errors"
	"path/filepath"
	"testing"

	storage "github.com/lazybark/cents/storage/sqlite"
)

// encryptionOptions wire encryption like the app does (see cli).
func encryptionOptions(opts Options) Options {
	opts.Choose = func(path string, create bool) (StorageWorker, string, error) {
		if err := storage.CheckDatabase(path); errors.Is(err, storage.ErrEncrypted) {
			return nil, "", ErrLocked
		}

		db, _, err := storage.OpenDatabase(path)
		if err != nil {
			return nil, "", err
		}

		return storage.NewSQLiteStorage(db), path, nil
	}
	opts.Unlock = func(path, password string) (StorageWorker, error) {
		db, vault, err := storage.OpenEncrypted(path, password)
		if err != nil {
			return nil, err
		}

		return storage.NewEncryptedStorage(db, vault), nil
	}
	opts.Encrypt = func(s StorageWorker, path, password string) (StorageWorker, error) {
		return s.(*storage.SQLiteStorage).Encrypt(path, password)
	}
	opts.Decrypt = func(s StorageWorker, password string) (StorageWorker, error) {
		return s.(*storage.SQLiteStorage).Decrypt(password)
	}

	return opts
}

func TestPasswords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cents.db")
	db, _, err := storage.OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	api := newAPI(encryptionOptions(Options{Storage: storage.NewSQLiteStorage(db), DBPath: path}))
	api.rates = &fakeRates{err: errors.New("offline in tests")}
	if err := api.SaveCategory(CategoryInput{Name: "Food"}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "12", Date: "2026-03-01", Category: "Food", Comment: "secret"}); err != nil {
		t.Fatal(err)
	}

	for _, bad := range []PasswordInput{{New: ""}, {New: "short"}} {
		if err := api.SetPassword(bad); err == nil {
			t.Errorf("expected %+v refused", bad)
		}
	}

	if err := api.SetPassword(PasswordInput{New: "correct horse"}); err != nil {
		t.Fatal(err)
	}
	if !api.Status().Encrypted {
		t.Fatal("expected it encrypted")
	}
	if month, err := api.CashflowMonth("2026-03"); err != nil || len(month.Entries) != 1 {
		t.Fatalf("expected the entry still there: %+v %v", month.Entries, err)
	}

	if err := api.SetPassword(PasswordInput{Current: "wrong horse", New: "battery staple"}); err == nil {
		t.Fatal("expected the current password checked")
	}
	if err := api.SetPassword(PasswordInput{Current: "correct horse", New: "battery staple"}); err != nil {
		t.Fatal(err)
	}

	// Quit and start again: locked until the password is given.
	api.shutdown(nil)
	again := newAPI(encryptionOptions(Options{LockedPath: path}))
	again.rates = &fakeRates{err: errors.New("offline in tests")}
	if status := again.Status(); !status.Locked || status.Ready || status.LockedPath != path {
		t.Fatalf("expected locked: %+v", status)
	}
	if err := again.Unlock("correct horse"); err == nil {
		t.Fatal("expected the old password to fail")
	}
	if err := again.Unlock("battery staple"); err != nil {
		t.Fatal(err)
	}
	if status := again.Status(); status.Locked || !status.Ready || !status.Encrypted || status.DBPath != path {
		t.Fatalf("expected open: %+v", status)
	}
	if month, _ := again.CashflowMonth("2026-03"); len(month.Entries) != 1 || month.Entries[0].Comment != "secret" {
		t.Fatalf("expected the entry: %+v", month.Entries)
	}

	// Picking an encrypted database from another: it waits for its
	// password, the open one stays; cancelling goes back.
	other := filepath.Join(dir, "other.db")
	odb, _, err := storage.OpenDatabase(other)
	if err != nil {
		t.Fatal(err)
	}
	plainAPI := newAPI(encryptionOptions(Options{Storage: storage.NewSQLiteStorage(odb), DBPath: other}))
	plainAPI.pick = &fakePickers{open: path}
	if ok, err := plainAPI.OpenDatabase(); !ok || err != nil {
		t.Fatalf("expected the pick taken: %v", err)
	}
	if status := plainAPI.Status(); !status.Locked || !status.Ready || status.DBPath != other {
		t.Fatalf("expected locked with the other still open: %+v", status)
	}
	plainAPI.CancelUnlock()
	if plainAPI.Status().Locked {
		t.Fatal("expected the unlock cancelled")
	}

	// Removing the password: a plain file again.
	if err := again.SetPassword(PasswordInput{New: ""}); err == nil {
		t.Fatal("expected the current password needed")
	}
	if err := again.SetPassword(PasswordInput{Current: "battery staple"}); err != nil {
		t.Fatal(err)
	}
	if again.Status().Encrypted {
		t.Fatal("expected it plain")
	}
	if err := storage.CheckDatabase(path); err != nil {
		t.Fatalf("expected a plain database file: %v", err)
	}
	if month, _ := again.CashflowMonth("2026-03"); len(month.Entries) != 1 {
		t.Fatal("expected the entry kept")
	}
}
