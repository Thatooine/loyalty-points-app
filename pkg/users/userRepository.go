package users

import "context"

// UserRepository is the persistence port for User entities. Methods
// participate in an ambient transaction when one is present in the context
// (see sqlite.TransactionManager), and run against the pool otherwise.
type UserRepository interface {
	// Create persists a new user. An existing user with the same ID or
	// email results in errs.ErrAlreadyExists.
	Create(ctx context.Context, request CreateUserRequest) (*CreateUserResponse, error)

	// List returns all users, oldest first.
	List(ctx context.Context, request ListUsersRequest) (*ListUsersResponse, error)

	// GetByID returns the user with the given ID, or errs.ErrNotFound.
	GetByID(ctx context.Context, request GetUserByIDRequest) (*GetUserByIDResponse, error)
}

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
}

// ListUsersResponse is the response for List.
type ListUsersResponse struct {
	Users []User
}

// GetUserByIDRequest is the request for GetByID.
type GetUserByIDRequest struct {
	ID string
}

// GetUserByIDResponse is the response for GetByID.
type GetUserByIDResponse struct {
	User User
}
