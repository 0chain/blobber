package common

import "errors"

var (
	// ErrBadDataStore bad db operation
	ErrBadDataStore = errors.New("datastore: bad db operation")
)
