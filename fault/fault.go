// Package fault names kinds of failure that cross layers: storage reports
// them, the service passes them on, and transports answer each kind its
// own way.
package fault

import "errors"

// ErrNotFound matches (errors.Is) every error saying a record doesn't exist.
var ErrNotFound = errors.New("not found")

// NotFound says what wasn't found: NotFound("debt") is "debt not found".
type NotFound string

func (e NotFound) Error() string { return string(e) + " not found" }

func (e NotFound) Is(target error) bool { return target == ErrNotFound }
