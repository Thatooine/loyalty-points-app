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

	// ErrInsufficientBalance indicates a spend or adjustment would drive an
	// account balance below zero. The balance is left unchanged.
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrUnauthorized indicates the caller could not be authenticated — bad
	// credentials, or a missing/invalid/expired token. Returned uniformly for
	// unknown user and wrong password so callers cannot probe which emails
	// exist.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates the caller is authenticated but not permitted to
	// act on the target — e.g. transacting on an account they do not own.
	ErrForbidden = errors.New("forbidden")
)
