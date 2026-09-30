package desktop

import (
	"context"
	"errors"
	"strings"
)

// Unlock opens the encrypted database waiting for its password.
func (a *API) Unlock(password string) error {
	a.mu.Lock()
	path := a.locked
	a.mu.Unlock()

	if path == "" {
		return errors.New("no database is waiting for a password")
	}

	if a.opts.Unlock == nil {
		return errors.New("encrypted databases can't be opened here")
	}

	storage, err := a.opts.Unlock(path, password)
	if err != nil {
		return err
	}

	a.use(storage, path)
	return nil
}

// CancelUnlock gives up on the encrypted database waiting for its
// password; the one open before, if any, stays.
func (a *API) CancelUnlock() {
	a.mu.Lock()
	a.locked = ""
	a.mu.Unlock()
}

// PasswordInput sets, changes or removes the database's password. Current
// is the password it has (empty when it has none); New the one it gets,
// empty to remove it.
type PasswordInput struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (a *API) SetPassword(input PasswordInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	switch {
	case !storage.Encrypted() && input.New == "":
		return errors.New("type a password to protect the database with")
	case !storage.Encrypted():
		if a.opts.Encrypt == nil {
			return errors.New("databases can't be encrypted here")
		}

		encrypted, err := a.opts.Encrypt(storage, a.Status().DBPath, input.New)
		if err != nil {
			return err
		}

		a.mu.Lock()
		a.storage = encrypted
		a.mu.Unlock()
		return nil
	case strings.TrimSpace(input.Current) == "":
		return errors.New("type the current password")
	case input.New != "":
		return storage.ChangePassword(input.Current, input.New)
	default:
		if a.opts.Decrypt == nil {
			return errors.New("databases can't be decrypted here")
		}

		plain, err := a.opts.Decrypt(storage, input.Current)
		if err != nil {
			return err
		}

		a.mu.Lock()
		a.storage = plain
		a.mu.Unlock()
		return nil
	}
}

// shutdown saves and closes the database as the app quits.
func (a *API) shutdown(context.Context) {
	a.mu.Lock()
	storage := a.storage
	a.mu.Unlock()

	if storage != nil {
		_ = storage.Close()
	}
}
