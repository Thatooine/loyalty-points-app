package users

import "context"

// UserRepository is the persistence port for User entities. Methods
// participate in an ambient transaction when one is present in the context
// (see sql.TxManager), and run against the pool otherwise.
type UserRepository interface {
	// Create persists a new user. An existing user with the same ID or
	// email results in errs.ErrAlreadyExists.
	Create(ctx context.Context, request CreateUserRequest) (*CreateUserResponse, error)

	// List returns users oldest first, scoped to the caller: a caller granted
	// user:read:all sees every user, otherwise only their own record (the user
	// whose id is the caller).
	List(ctx context.Context, request ListUsersRequest) (*ListUsersResponse, error)

	// GetByID returns the user with the given ID, or errs.ErrNotFound. The read
	// is ownership-scoped on the id: unless the caller holds user:read:all they
	// may only read their own record, so another user's id reads as
	// errs.ErrNotFound.
	GetByID(ctx context.Context, request GetUserByIDRequest) (*GetUserByIDResponse, error)

	// GetByEmail returns the user with the given email, or errs.ErrNotFound.
	// Used by authentication to resolve a login identity to its credentials.
	// Ownership-scoped on the id like GetByID: a caller holding user:read:all —
	// or the RootUserID principal used by login — reads any user, otherwise the
	// lookup is restricted to the caller's own record.
	GetByEmail(ctx context.Context, request GetUserByEmailRequest) (*GetUserByEmailResponse, error)

	// GetTokenVersion returns the user's current token_version (their session
	// epoch), or errs.ErrNotFound. It is the lean read on the token-validation
	// hot path, so it is NOT ownership-scoped: it is a trusted, server-internal
	// lookup keyed by the exact user id taken from an already signature-verified
	// token claim, never from client input.
	GetTokenVersion(ctx context.Context, request GetTokenVersionRequest) (*GetTokenVersionResponse, error)

	// IncrementTokenVersion atomically bumps the user's token_version by one and
	// returns the new value, or errs.ErrNotFound when no such user exists. This
	// is the revocation lever: every access token issued before the bump
	// carries a now-stale version and is rejected at validation. Like
	// GetTokenVersion it is a trusted, server-internal operation keyed by an
	// exact user id, not ownership-scoped.
	IncrementTokenVersion(ctx context.Context, request IncrementTokenVersionRequest) (*IncrementTokenVersionResponse, error)
}

// SystemUserID identifies the system principal used for unauthenticated,
// server-initiated lookups — chiefly the login flow, which must resolve a user
// by email before any login claim exists. The user repository treats it as
// exempt from ownership scoping. It is set only by trusted server code and must
// never be populated from client input.

// CreateUserRequest is the request for Create.
type CreateUserRequest struct {
	User User
}

// CreateUserResponse is the response for Create.
type CreateUserResponse struct {
	User User
}

// ListUsersRequest is the request for List.
type ListUsersRequest struct {
	// UserID is the calling user's id. Unless the caller holds user:read:all the
	// listing is restricted to this id (their own record).
	UserID string
}

// ListUsersResponse is the response for List.
type ListUsersResponse struct {
	Users []User
}

// GetUserByIDRequest is the request for GetByID.
type GetUserByIDRequest struct {
	// ID is the user to fetch.
	ID string

	// UserID is the calling user's id. Unless the caller holds user:read:all the
	// lookup additionally requires id == UserID, so a caller can only read their
	// own record.
	UserID string
}

// GetUserByIDResponse is the response for GetByID.
type GetUserByIDResponse struct {
	User User
}

// GetUserByEmailRequest is the request for GetByEmail.
type GetUserByEmailRequest struct {
	Email string

	// UserID is the calling principal's id. Unless the caller holds user:read:all
	// or is the RootUserID, the lookup is restricted to their own record
	// (id == UserID). The login flow passes RootUserID.
	UserID string
}

// GetUserByEmailResponse is the response for GetByEmail.
type GetUserByEmailResponse struct {
	User User
}

// GetTokenVersionRequest is the request for GetTokenVersion.
type GetTokenVersionRequest struct {
	// UserID is the user whose token_version to read. It comes from a verified
	// token claim, not client input.
	UserID string
}

// GetTokenVersionResponse is the response for GetTokenVersion.
type GetTokenVersionResponse struct {
	TokenVersion int64
}

// IncrementTokenVersionRequest is the request for IncrementTokenVersion.
type IncrementTokenVersionRequest struct {
	// UserID is the user whose token_version to bump (the principal logging out).
	UserID string
}

// IncrementTokenVersionResponse is the response for IncrementTokenVersion.
type IncrementTokenVersionResponse struct {
	// TokenVersion is the new, post-increment value.
	TokenVersion int64
}
