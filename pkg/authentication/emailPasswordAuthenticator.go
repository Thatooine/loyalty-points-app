package authentication

import "context"

// EmailPasswordAuthenticator authenticates users with an email and password,
// returning an access token on success.
type EmailPasswordAuthenticator interface {
	Authenticate(ctx context.Context, request EmailPasswordAuthenticatorRequest) (*EmailPasswordAuthenticatorResponse, error)
}

type EmailPasswordAuthenticatorRequest struct {
	Email    string
	Password string
}

type EmailPasswordAuthenticatorResponse struct {
	Token  string
	UserID string
	Email  string
}
