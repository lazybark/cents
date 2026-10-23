package desktop

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/lazybark/cents/client"
	"github.com/lazybark/cents/dbimport"
	"github.com/lazybark/cents/service"
)

type DatabaseImportPreview struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Destination string          `json:"destination"`
	Result      dbimport.Result `json:"result"`
}

type pendingDatabaseImport struct {
	preview   DatabaseImportPreview
	data      []byte
	importer  service.DatabaseImporter
	storage   StorageWorker
	server    *Server
	connected bool
}

// PreviewDatabaseImport uses a native file picker and stages a fixed snapshot.
// Confirming imports those exact bytes, into the same destination as previewed.
func (a *API) PreviewDatabaseImport() (DatabaseImportPreview, error) {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	a.pendingImport = nil
	a.mu.Lock()
	stage, err := a.importTargetLocked()
	currentPath := a.dbPath
	a.mu.Unlock()
	if err != nil {
		return DatabaseImportPreview{}, err
	}
	dir, err := a.saveDir()
	if err != nil {
		return DatabaseImportPreview{}, err
	}
	path, err := a.pickers().OpenFile("Import records from a cents database", dir, "")
	if err != nil {
		return DatabaseImportPreview{}, err
	}
	if path == "" {
		return DatabaseImportPreview{}, nil
	}
	if sameFile(path, currentPath) {
		return DatabaseImportPreview{}, errors.New("choose a backup rather than the database currently open")
	}
	f, err := os.Open(path)
	if err != nil {
		return DatabaseImportPreview{}, err
	}
	data, readErr := io.ReadAll(io.LimitReader(f, dbimport.MaxBytes+1))
	closeErr := f.Close()
	if int64(len(data)) > dbimport.MaxBytes {
		return DatabaseImportPreview{}, dbimport.ErrTooLarge
	}
	if readErr != nil {
		return DatabaseImportPreview{}, readErr
	}
	if closeErr != nil {
		return DatabaseImportPreview{}, closeErr
	}
	result, err := stage.importer.ImportDatabase(a.importContext(), bytes.NewReader(data), true)
	if err != nil {
		return DatabaseImportPreview{}, err
	}
	stage.preview.ID, stage.preview.Name, stage.preview.Result = uuid.NewString(), filepath.Base(path), result
	stage.data = data
	a.mu.Lock()
	defer a.mu.Unlock()
	if !stage.sameTarget(a) {
		return DatabaseImportPreview{}, errors.New("the database changed; preview the import again")
	}
	a.pendingImport = stage
	return stage.preview, nil
}

// ImportDatabase commits a previously previewed upload, without exposing local
// paths or accepting new file contents from the frontend at confirmation time.
func (a *API) ImportDatabase(previewID string) (dbimport.Result, error) {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	stage := a.pendingImport
	if stage == nil || stage.preview.ID != previewID {
		return dbimport.Result{}, errors.New("preview a database before importing")
	}
	if !stage.sameTarget(a) {
		a.pendingImport = nil
		return dbimport.Result{}, errors.New("the database changed; preview the import again")
	}
	result, err := stage.importer.ImportDatabase(a.importContext(), bytes.NewReader(stage.data), false)
	if err == nil {
		a.pendingImport = nil
	}
	return result, err
}

func (a *API) CancelDatabaseImport(previewID string) {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	if a.pendingImport != nil && a.pendingImport.preview.ID == previewID {
		a.pendingImport = nil
	}
}

func (a *API) importContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (p *pendingDatabaseImport) sameTarget(a *API) bool {
	return p.storage == a.storage && p.server == a.server && p.connected == a.connected
}

// Caller holds a.mu. Use a fixed backend so switches during preview cannot
// redirect either operation to a different local database or remote server.
func (a *API) importTargetLocked() (*pendingDatabaseImport, error) {
	p := &pendingDatabaseImport{storage: a.storage, server: a.server, connected: a.connected}
	if a.connected && a.server != nil {
		options := []client.Option{client.WithHTTPClient(&http.Client{Timeout: requestTimeout})}
		if a.server.Certificate != "" {
			options = append(options, client.WithCertificate(a.server.Certificate))
		}
		remote, err := client.New(a.server.URL, a.server.Token, options...)
		if err != nil {
			return nil, err
		}
		p.importer, p.preview.Destination = remote, a.server.URL
	} else if a.storage != nil {
		if a.storage.Encrypted() {
			return nil, errors.New("import into an unencrypted database or server; encrypted destinations are not supported yet")
		}
		p.importer, p.preview.Destination = service.On(a.storage), a.dbPath
	} else {
		return nil, service.ErrNoDatabase
	}
	return p, nil
}
