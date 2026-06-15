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
	// or the SystemUserID principal used by login — reads any user, otherwise the
	// lookup is restricted to the caller's own record.
	GetByEmail(ctx context.Context, request GetUserByEmailRequest) (*GetUserByEmailResponse, error)
}

// SystemUserID identifies the system principal used for unauthenticated,
// server-initiated lookups — chiefly the login flow, which must resolve a user
// by email before any login claim exists. The user repository treats it as
// exempt from ownership scoping. It is set only by trusted server code and must
// never be populated from client input.
const SystemUserID = "system"

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
	// or is the SystemUserID, the lookup is restricted to their own record
	// (id == UserID). The login flow passes SystemUserID.
	UserID string
}

// GetUserByEmailResponse is the response for GetByEmail.
type GetUserByEmailResponse struct {
	User User
}
