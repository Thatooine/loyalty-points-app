package errs

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNotFound indicates the requested record does not exist.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates a record with the same natural key already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrDuplicateRef indicates a transaction with the same idempotency ref has
	// already been recorded. Callers treat this as a duplicate submission, not a
	// failure.
	ErrDuplicateRef = errors.New("duplicate ref")

	// ErrInsufficientBalance indicates a spend would drive an account balance
	// below zero. The balance is left unchanged.
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrUnauthorized indicates the caller could not be authenticated — bad
	// credentials, or a missing/invalid/expired token. Returned uniformly for
	// unknown user and wrong password so callers cannot probe which emails
	// exist.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates the caller is authenticated but not permitted to
	// act on the target — e.g. transacting on an account they do not own.
	ErrForbidden = errors.New("forbidden")

	// ErrInvalidArgument indicates a caller-supplied value was malformed — e.g.
	// an unparseable pagination cursor. It is a client error, not a server fault.
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrInternal indicates an unexpected server-side failure. It is the
	// deliberate, client-safe stand-in for an internal error: adaptors wrap it
	// with a friendly message (via WithMessage) so the cause stays in the logs
	// while the client sees only the safe summary.
	ErrInternal = errors.New("internal error")
)

// ValidationError is a client error carrying the individual reasons a request
// was rejected. It unwraps to ErrInvalidArgument so callers can match the class
// with errors.Is while still surfacing the per-field detail to the client.
type ValidationError struct {
	Reasons []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %s", strings.Join(e.Reasons, "; "))
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidArgument
}

// NewValidationError builds a ValidationError from a set of reasons. It returns
// nil when there are none, so callers can return its result unconditionally.
func NewValidationError(reasons []string) error {
	if len(reasons) == 0 {
		return nil
	}
	return &ValidationError{Reasons: reasons}
}

// WithMessage pairs a client-facing message with a sentinel: the returned
// error's Error() is msg, while Unwrap() exposes sentinel so errors.Is/As keep
// working. It lets an adaptor attach contextual phrasing ("account not found")
// without losing the sentinel that drives the JSON-RPC error code.
func WithMessage(sentinel error, msg string) error {
	return &messageError{sentinel: sentinel, msg: msg}
}

type messageError struct {
	sentinel error
	msg      string
}

func (e *messageError) Error() string { return e.msg }
func (e *messageError) Unwrap() error { return e.sentinel }
