package common

import "errors"

var (
	// ErrBadDataStore bad db operation
	ErrBadDataStore = errors.New("datastore: bad db operation")

	// ErrInvalidParameter parameter is not specified or invalid
	ErrInvalidParameter = errors.New("invalid parameter")

	// ErrUnableHash failed to hash with unknown exception
	ErrUnableHash = errors.New("unable to hash")

	// ErrUnableWriteFile failed to write bytes to file
	ErrUnableWriteFile = errors.New("unable to write file")

	// ErrNotImplemented feature/method is not implemented yet
	ErrNotImplemented = errors.New("not implemented")

	// ErrInvalidOperation failed to invoke a method
	ErrInvalidOperation = errors.New("invalid operation")

	// ErrBadRequest bad request
	ErrBadRequest = errors.New("bad request")

	// ErrUnknown unknown exception
	ErrUnknown = errors.New("unknown")

	// ErrInternal an unknown internal server error
	ErrInternal = errors.New("internal")

	// ErrEntityNotFound entity can't found in db
	ErrEntityNotFound = errors.New("entity not found")

	// ErrMissingRootNode  root node is missing
	ErrMissingRootNode = errors.New("root node is missing")

	// ErrDuplicatedNode  duplicated nodes
	ErrDuplicatedNode = errors.New("duplicated nodes")
	// ErrFileWasDeleted file already was deleted
	ErrFileWasDeleted = errors.New("file was deleted")

	// ErrNotFound ref is not found
	ErrNotFound = errors.New("ref is not found")

	// ErrNoThumbnail no thumbnail
	ErrNoThumbnail = errors.New("no thumbnail")
)
