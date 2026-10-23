package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybark/cents/service"
)

func importBackup(t *testing.T) string {
	t.Helper()
	source := newTestAPI(t)
	if err := source.CreateAccount(service.NewAccountInput{Name: "Imported from desktop", Description: "Seed account", Currency: "$", Amount: "123.45"}); err != nil {
		t.Fatal(err)
	}
	store, err := source.currentStorage()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	path := filepath.Join(t.TempDir(), "seed.db")
	if err := store.Backup(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDesktopImportPreviewConfirmAndCancel(t *testing.T) {
	api := newTestAPI(t)
	t.Cleanup(func() { api.shutdown(nil) })
	pickers := &fakePickers{}
	api.pick = pickers
	if preview, err := api.PreviewDatabaseImport(); err != nil || preview.ID != "" {
		t.Fatalf("cancel file dialog: %+v %v", preview, err)
	}
	pickers.open = api.Status().DBPath
	if _, err := api.PreviewDatabaseImport(); err == nil || !strings.Contains(err.Error(), "currently open") {
		t.Fatalf("self import: %v", err)
	}
	path := importBackup(t)
	pickers.open = path
	preview, err := api.PreviewDatabaseImport()
	if err != nil || preview.ID == "" || preview.Result.Added == 0 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if accountNames(t, api) != "" {
		t.Fatal("preview added an account")
	}
	api.CancelDatabaseImport(preview.ID)
	if _, err := api.ImportDatabase(preview.ID); err == nil {
		t.Fatal("cancelled import was still usable")
	}
	preview, err = api.PreviewDatabaseImport()
	if err != nil {
		t.Fatal(err)
	}
	// Confirmation must use the exact bytes previewed, even if the original
	// file gets changed or removed while the preview dialog is open.
	if err := os.WriteFile(path, []byte("replaced after preview"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := api.ImportDatabase(preview.ID)
	if err != nil || result.Added != preview.Result.Added {
		t.Fatalf("confirm: %+v %v", result, err)
	}
	if accountNames(t, api) != "Imported from desktop" {
		t.Fatal("account was not imported")
	}
	if _, err := api.ImportDatabase(preview.ID); err == nil {
		t.Fatal("replayed preview was accepted")
	}
}

func TestDesktopImportRefusesChangedDestination(t *testing.T) {
	api := newTestAPI(t)
	other := newTestAPI(t)
	original := api.storage
	t.Cleanup(func() { _ = original.Close(); _ = other.storage.Close() })
	api.pick = &fakePickers{open: importBackup(t)}
	preview, err := api.PreviewDatabaseImport()
	if err != nil {
		t.Fatal(err)
	}
	api.mu.Lock()
	api.storage = other.storage
	api.mu.Unlock()
	if _, err := api.ImportDatabase(preview.ID); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed destination: %v", err)
	}
	if accountNames(t, other) != "" {
		t.Fatal("imported into the wrong database")
	}
}

func TestDesktopImportIntoServer(t *testing.T) {
	srv, token := testServer(t, false)
	api := newAPI(Options{})
	if _, err := api.ConnectServer(ServerInput{URL: srv.URL, Token: token}); err != nil {
		t.Fatal(err)
	}
	api.pick = &fakePickers{open: importBackup(t)}
	preview, err := api.PreviewDatabaseImport()
	if err != nil || preview.Destination != srv.URL {
		t.Fatalf("remote preview: %+v %v", preview, err)
	}
	if accountNames(t, api) != "On the server" {
		t.Fatal("remote preview changed data")
	}
	if result, err := api.ImportDatabase(preview.ID); err != nil || result.Added == 0 {
		t.Fatalf("remote import: %+v %v", result, err)
	}
	if names := accountNames(t, api); !strings.Contains(names, "On the server") || !strings.Contains(names, "Imported from desktop") {
		t.Fatalf("remote records: %s", names)
	}
	preview, err = api.PreviewDatabaseImport()
	if err != nil || preview.Result.Added != 0 {
		t.Fatalf("repeat remote import: %+v %v", preview, err)
	}
	api.CancelDatabaseImport(preview.ID)
}
