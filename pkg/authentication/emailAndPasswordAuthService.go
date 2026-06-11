package authentication

import "context"

// EmailAndPasswordAuthService authenticates users with an email and password,
// returning an access token on success.
type EmailAndPasswordAuthService interface {
	Authenticate(ctx context.Context, request EmailAndPasswordAuthRequest) (*EmailAndPasswordAuthResponse, error)
}

// EmailAndPasswordAuthRequest is the request for Authenticate.
type EmailAndPasswordAuthRequest struct {
	Email    string
	Password string
}

// EmailAndPasswordAuthResponse is the response for Authenticate.
type EmailAndPasswordAuthResponse struct {
	Token  string
	UserID string
	Email  string
}
