package service

import (
	"errors"

	"github.com/lazybark/cents/fault"
)

// A call fails with an error that errors.Is ErrNotFound when the record it
// names doesn't exist, and ErrUpstream when the exchange rate source
// failed. Any other error is either the caller's mistake (its message says
// what to fix) or the database's; a transport tells those apart with
// help from the storage.
var (
	ErrNotFound = fault.ErrNotFound
	ErrUpstream = errors.New("the exchange rate source failed")
)

// notFoundError names what wasn't found: "debt not found".
type notFoundError = fault.NotFound

// upstreamError keeps the rate source's message.
type upstreamError struct{ err error }

func (e upstreamError) Error() string { return e.err.Error() }

func (e upstreamError) Unwrap() []error { return []error{e.err, ErrUpstream} }
