package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/rates"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func newTestAPI(t *testing.T) *API {
	t.Helper()

	path := filepath.Join(t.TempDir(), "cents.db")
	db, _, err := storage.OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.08}).Error; err != nil {
		t.Fatal(err)
	}

	return offline(newAPI(Options{Storage: storage.NewSQLiteStorage(db), DBPath: path}))
}

// offline keeps api's rate fetches off the network.
func offline(api *API) *API {
	api.svc.WithRates(offlineRates{})
	return api
}

type offlineRates struct{}

func (offlineRates) Fetch(context.Context, string) (rates.Quote, error) {
	return rates.Quote{}, errors.New("offline in tests")
}

// fakePickers answers dialogs with fixed paths ("" is cancelling).
type fakePickers struct {
	file, open, folder string
	dirs               []string
}

func (f *fakePickers) SaveFile(_, dir, _ string) (string, error) {
	f.dirs = append(f.dirs, dir)
	return f.file, nil
}

func (f *fakePickers) OpenFile(_, dir, _ string) (string, error) {
	f.dirs = append(f.dirs, dir)
	return f.open, nil
}

func (f *fakePickers) Folder(_, dir string) (string, error) {
	f.dirs = append(f.dirs, dir)
	return f.folder, nil
}

func (a *API) storageAndSettingsForTest() (settings.AppSettings, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return settings.AppSettings{}, err
	}

	return storage.LoadAppSettings()
}
