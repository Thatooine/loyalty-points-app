package authentication

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

type EmailPasswordAuthenticatorJSONRPCAdaptor struct {
	authService EmailPasswordAuthenticator
}

func NewEmailPasswordAuthenticatorJSONRPCAdaptor(
	authService EmailPasswordAuthenticator,
) *EmailPasswordAuthenticatorJSONRPCAdaptor {
	return &EmailPasswordAuthenticatorJSONRPCAdaptor{
		authService: authService,
	}
}

func (a *EmailPasswordAuthenticatorJSONRPCAdaptor) Name() string {
	return "EmailPasswordAuthenticator"
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"userID"`
	Email  string `json:"email"`
}

func (a *EmailPasswordAuthenticatorJSONRPCAdaptor) Login(r *http.Request, request *LoginRequest, response *LoginResponse) error {
	authResp, err := a.authService.Authenticate(r.Context(), EmailPasswordAuthenticatorRequest{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		log.Ctx(r.Context()).Error().Err(err).Msg("failed to authenticate with email and password")
		return errors.New("invalid credentials")
	}

	response.Token = authResp.Token
	response.UserID = authResp.UserID
	response.Email = authResp.Email
	return nil
}
