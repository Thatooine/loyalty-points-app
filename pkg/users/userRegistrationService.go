package users

import "context"

// UserRegistrationService onboards a new principal: it creates the user
// identity and opens their first wallet account as a single atomic flow, then
// issues an access token so the caller is logged in straight after signing up.
type UserRegistrationService interface {
	// Register creates a user and opens an account for them in one unit of
	// work, returning an access token. An email that is already registered
	// results in errs.ErrAlreadyExists and no partial state.
	Register(ctx context.Context, request RegisterRequest) (*RegisterResponse, error)
}

// RegisterRequest is the request for Register.
type RegisterRequest struct {
	Email    string
	Password string
	Name     string

	// AccountName names the wallet account opened during registration. When
	// empty it defaults to "Primary Wallet".
	AccountName string
}

// RegisterResponse is the response for Register.
type RegisterResponse struct {
	Token     string
	UserID    string
	AccountID string
	Email     string
}
