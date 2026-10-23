// Package dbimport describes a merge from a cents database into another one.
package dbimport

import "errors"

const MaxBytes int64 = 32 << 20
const MaxRows = 100000

var ErrTooLarge = errors.New("the database is too large; choose a file up to 32 MB")

type Result struct {
	Preview   bool    `json:"preview"`
	Added     int     `json:"added"`
	Skipped   int     `json:"skipped"`
	Conflicts int     `json:"conflicts"`
	Tables    []Table `json:"tables"`
}

type Table struct {
	Name      string `json:"name"`
	Added     int    `json:"added"`
	Skipped   int    `json:"skipped"`
	Conflicts int    `json:"conflicts"`
}
