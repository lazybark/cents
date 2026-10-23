package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lazybark/cents/dbimport"
)

// DatabaseImporter is a file transfer capability, separate from JSON Backend.
type DatabaseImporter interface {
	ImportDatabase(context.Context, io.Reader, bool) (dbimport.Result, error)
}

func (s *Service) ImportDatabase(ctx context.Context, reader io.Reader, preview bool) (dbimport.Result, error) {
	store, err := s.currentStorage()
	if err != nil {
		return dbimport.Result{}, err
	}
	importer, ok := store.(interface {
		ImportDatabase(context.Context, string, bool) (dbimport.Result, error)
	})
	if !ok {
		return dbimport.Result{}, errors.New("this database does not support importing")
	}
	dir, err := os.MkdirTemp("", "cents-import-")
	if err != nil {
		return dbimport.Result{}, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "upload.db")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return dbimport.Result{}, err
	}
	n, copyErr := io.Copy(f, io.LimitReader(reader, dbimport.MaxBytes+1))
	closeErr := f.Close()
	if n > dbimport.MaxBytes {
		return dbimport.Result{}, dbimport.ErrTooLarge
	}
	if copyErr != nil {
		return dbimport.Result{}, fmt.Errorf("could not receive the database: %w", copyErr)
	}
	if closeErr != nil {
		return dbimport.Result{}, closeErr
	}
	if err := ctx.Err(); err != nil {
		return dbimport.Result{}, err
	}
	return importer.ImportDatabase(ctx, path, preview)
}
