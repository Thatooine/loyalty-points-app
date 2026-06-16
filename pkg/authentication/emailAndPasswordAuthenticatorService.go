package authentication

import "context"

type EmailAndPasswordAuthenticatorService interface {
	AuthenticateWithEmailAndPassword(ctx context.Context, request EmailAndPasswordAuthRequest) (*EmailAndPasswordAuthResponse, error)
}

type EmailAndPasswordAuthRequest struct {
	Email    string
	Password string
}

type EmailAndPasswordAuthResponse struct {
	Token  string
	UserID string
	Email  string
}
