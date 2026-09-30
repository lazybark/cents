package desktop

import (
	"path/filepath"
	"testing"

	"github.com/lazybark/cents/flows/settings"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func TestSwitchingDatabases(t *testing.T) {
	api := newTestAPI(t)
	api.dbPath = filepath.Join(t.TempDir(), "cents.db")
	old, _ := api.currentStorage()

	other := filepath.Join(t.TempDir(), "other.db")
	db, _, err := storage.OpenDatabase(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&settings.SettingIncomeCategory{CategoryName: "Only here"}).Error; err != nil {
		t.Fatal(err)
	}

	api.opts.Choose = func(path string, create bool) (StorageWorker, string, error) {
		db, _, err := storage.OpenDatabase(path)
		if err != nil {
			return nil, "", err
		}

		return storage.NewSQLiteStorage(db), path, nil
	}

	pick := &fakePickers{open: other}
	api.pick = pick
	before := api.Status().DBPath
	if ok, err := api.OpenDatabase(); !ok || err != nil {
		t.Fatalf("switch failed: %v", err)
	}

	if pick.dirs[0] != filepath.Dir(before) {
		t.Fatalf("expected the dialog in the current database's folder: %v", pick.dirs)
	}

	if status := api.Status(); status.DBPath != other || !status.Ready {
		t.Fatalf("unexpected status %+v", status)
	}

	stts, err := api.storageAndSettingsForTest()
	if err != nil || len(stts.IncomeCategories) != 1 || stts.IncomeCategories[0].CategoryName != "Only here" {
		t.Fatalf("expected the other database's settings: %+v %v", stts.IncomeCategories, err)
	}

	if _, err := old.LoadAccounts(); err == nil {
		t.Fatal("expected the previous database closed")
	}

	// Creating one starts next to the database in use; the app's own
	// chooser (cli) refuses an existing file, which this test's doesn't.
	created := filepath.Join(filepath.Dir(other), "fresh.db")
	pick.file = created
	if ok, err := api.CreateDatabase(); !ok || err != nil || api.Status().DBPath != created || !api.Status().NeedsCurrencies {
		t.Fatalf("expected the new database, needing currencies: %+v %v", api.Status(), err)
	}

	if pick.dirs[1] != filepath.Dir(other) {
		t.Fatalf("expected the dialog next to the database in use: %v", pick.dirs)
	}

	if ok, err := api.choose("", false); ok || err != nil {
		t.Fatal("an empty pick (cancelled) changes nothing")
	}
}

func (a *API) storageAndSettingsForTest() (settings.AppSettings, error) {
	_, stts, err := a.storageAndSettings()
	return stts, err
}
