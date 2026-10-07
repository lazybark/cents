package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybark/cents/service"
)

func TestBackupDatabase(t *testing.T) {
	api := newTestAPI(t)
	dir := t.TempDir()
	pick := &fakePickers{}
	api.pick = pick

	if got, err := api.BackupDatabase(); err != nil || got.Path != "" {
		t.Fatalf("cancelling does nothing: %+v %v", got, err)
	}

	if pick.dirs[0] != filepath.Dir(api.Status().DBPath) {
		t.Fatalf("expected the first backup next to the database: %v", pick.dirs)
	}

	// The database itself is refused, with or without its extension.
	for _, self := range []string{api.Status().DBPath, strings.TrimSuffix(api.Status().DBPath, ".db")} {
		pick.file = self
		if _, err := api.BackupDatabase(); err == nil || !strings.Contains(err.Error(), "the database itself") {
			t.Fatalf("expected %s refused, got %v", self, err)
		}
	}

	pick.file = filepath.Join(dir, "my-backup")
	got, err := api.BackupDatabase()
	if err != nil || got.Path != filepath.Join(dir, "my-backup.db") {
		t.Fatalf("unexpected %+v %v", got, err)
	}

	if info, err := os.Stat(got.Path); err != nil || info.Size() == 0 {
		t.Fatalf("expected the backup file: %v", err)
	}

	view, err := api.ExportOptions()
	if err != nil || view.DBPath != api.Status().DBPath || view.LastBackupPath != got.Path {
		t.Fatalf("expected the database and the backup: %+v %v", view, err)
	}

	// The next backup starts in the folder of the last one.
	if _, err := api.BackupDatabase(); err != nil || pick.dirs[len(pick.dirs)-1] != dir {
		t.Fatalf("expected the last folder: %v %v", pick.dirs, err)
	}
}

func TestExportCSVDialog(t *testing.T) {
	api := newTestAPI(t)
	pick := &fakePickers{}
	api.pick = pick

	// Mistakes are caught before the dialog.
	if _, err := api.ExportCSV(service.ExportInput{From: "01.03.2026", Datasets: []string{"debts"}}); err == nil || len(pick.dirs) != 0 {
		t.Fatalf("expected the date refused before the dialog: %v %v", err, pick.dirs)
	}

	if got, err := api.ExportCSV(service.ExportInput{Datasets: []string{"debts"}}); err != nil || got.Folder != "" {
		t.Fatalf("cancelling does nothing: %+v %v", got, err)
	}

	pick.folder = t.TempDir()
	got, err := api.ExportCSV(service.ExportInput{Datasets: []string{"debts", "net_worth"}})
	if err != nil || filepath.Dir(got.Folder) != pick.folder || len(got.Files) != 2 || got.Files[0].Name != "net_worth.csv" {
		t.Fatalf("unexpected %+v %v", got, err)
	}
}

func TestSettingsShowTheDatabase(t *testing.T) {
	api := newTestAPI(t)

	view, err := api.Settings()
	if err != nil || view.DBPath != api.Status().DBPath || view.DBPath == "" || len(view.Currencies) != 1 {
		t.Fatalf("unexpected %+v %v", view, err)
	}

	if err := api.OpenInvoiceURL(99); err == nil {
		t.Fatal("expected a missing invoice refused")
	}
}
