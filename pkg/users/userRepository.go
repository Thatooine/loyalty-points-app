package users

import "context"

type UserRepository interface {
	Create(ctx context.Context, request CreateUserRequest) (*CreateUserResponse, error)

	List(ctx context.Context, request ListUsersRequest) (*ListUsersResponse, error)

	GetByID(ctx context.Context, request GetUserByIDRequest) (*GetUserByIDResponse, error)

	GetByEmail(ctx context.Context, request GetUserByEmailRequest) (*GetUserByEmailResponse, error)

	// GetTokenVersion is NOT ownership-scoped: it is keyed by a user id taken
	// from an already signature-verified token claim, never from client input.
	GetTokenVersion(ctx context.Context, request GetTokenVersionRequest) (*GetTokenVersionResponse, error)

	// IncrementTokenVersion is NOT ownership-scoped, for the same reason as
	// GetTokenVersion; bumping the version is the token-revocation lever.
	IncrementTokenVersion(ctx context.Context, request IncrementTokenVersionRequest) (*IncrementTokenVersionResponse, error)
}

type CreateUserRequest struct {
	User User
}

type CreateUserResponse struct {
	User User
}

type ListUsersRequest struct {
	UserID string
}

type ListUsersResponse struct {
	Users []User
}

type GetUserByIDRequest struct {
	ID string

	UserID string
}

type GetUserByIDResponse struct {
	User User
}

type GetUserByEmailRequest struct {
	Email string

	UserID string
}

type GetUserByEmailResponse struct {
	User User
}

type GetTokenVersionRequest struct {
	UserID string
}

type GetTokenVersionResponse struct {
	TokenVersion int64
}

type IncrementTokenVersionRequest struct {
	UserID string
}

type IncrementTokenVersionResponse struct {
	TokenVersion int64
}
