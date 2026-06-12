package errs

import "errors"

var (
	// ErrNotFound indicates the requested record does not exist.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates a record with the same natural key already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrDuplicateRef indicates a transaction with the same idempotency ref has
	// already been recorded. Callers treat this as a duplicate submission, not a
	// failure.
	ErrDuplicateRef = errors.New("duplicate ref")
)
