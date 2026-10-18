package sqlite

import (
	"errors"
	"io/fs"
	"os"

	"github.com/lazybark/cents/fault"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

// Kinds of storage error a transport answers differently: what's missing,
// what conflicts with records there, and the database's own failures,
// whose details stay on the server. Any other error refuses what was asked
// in words meant for the user.
const (
	Refused = iota
	Missing
	Conflict
	Failure
)

// gormFailures are gorm's errors for a query gone wrong.
var gormFailures = []error{
	gorm.ErrInvalidTransaction, gorm.ErrNotImplemented, gorm.ErrMissingWhereClause, gorm.ErrUnsupportedRelation,
	gorm.ErrPrimaryKeyRequired, gorm.ErrModelValueRequired, gorm.ErrModelAccessibleFieldsRequired, gorm.ErrSubQueryRequired,
	gorm.ErrInvalidData, gorm.ErrUnsupportedDriver, gorm.ErrRegistered, gorm.ErrInvalidField, gorm.ErrEmptySlice,
	gorm.ErrDryRunModeUnsupported, gorm.ErrInvalidDB, gorm.ErrInvalidValue, gorm.ErrInvalidValueOfLength, gorm.ErrPreloadNotAllowed,
}

// Kind says which kind of storage error err is.
func Kind(err error) int {
	var inUse InUseError
	var sqliteErr sqlite3.Error
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	var syscallErr *os.SyscallError

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, fault.ErrNotFound):
		return Missing
	case errors.As(err, &inUse), errors.Is(err, gorm.ErrDuplicatedKey):
		return Conflict
	case errors.As(err, &sqliteErr):
		if sqliteErr.Code == sqlite3.ErrConstraint {
			return Conflict
		}

		return Failure
	case errors.As(err, &pathErr), errors.As(err, &linkErr), errors.As(err, &syscallErr):
		return Failure
	}

	for _, failure := range gormFailures {
		if errors.Is(err, failure) {
			return Failure
		}
	}

	return Refused
}
