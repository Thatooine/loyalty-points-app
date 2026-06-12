package authentication

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

type EmailAndPasswordAuthJSONRPCAdaptor struct {
	authService EmailAndPasswordAuthService
}

func NewEmailAndPasswordAuthJSONRPCAdaptor(
	authService EmailAndPasswordAuthService,
) *EmailAndPasswordAuthJSONRPCAdaptor {
	return &EmailAndPasswordAuthJSONRPCAdaptor{
		authService: authService,
	}
}

func (a *EmailAndPasswordAuthJSONRPCAdaptor) Name() string {
	return "EmailAndPasswordAuth"
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

func (a *EmailAndPasswordAuthJSONRPCAdaptor) Login(r *http.Request, request *LoginRequest, response *LoginResponse) error {
	authResp, err := a.authService.Authenticate(r.Context(), EmailAndPasswordAuthRequest{
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
