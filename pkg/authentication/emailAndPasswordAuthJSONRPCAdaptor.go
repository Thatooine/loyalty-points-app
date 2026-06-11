package authentication

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

// EmailAndPasswordAuthJSONRPCAdaptor exposes email/password authentication
// over JSON-RPC.
type EmailAndPasswordAuthJSONRPCAdaptor struct {
	authService EmailAndPasswordAuthService
}

// NewEmailAndPasswordAuthJSONRPCAdaptor returns a new
// EmailAndPasswordAuthJSONRPCAdaptor wrapping the given auth service.
func NewEmailAndPasswordAuthJSONRPCAdaptor(
	authService EmailAndPasswordAuthService,
) *EmailAndPasswordAuthJSONRPCAdaptor {
	return &EmailAndPasswordAuthJSONRPCAdaptor{
		authService: authService,
	}
}

// Name returns the name under which this service is registered with the
// JSON-RPC server. Methods are invoked as "EmailAndPasswordAuth.<Method>".
func (a *EmailAndPasswordAuthJSONRPCAdaptor) Name() string {
	return "EmailAndPasswordAuth"
}

// LoginRequest is the JSON-RPC request for EmailAndPasswordAuth.Login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is the JSON-RPC response after a successful login.
type LoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"userID"`
	Email  string `json:"email"`
}

// Login authenticates a user with email and password and returns a signed
// access token.
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
