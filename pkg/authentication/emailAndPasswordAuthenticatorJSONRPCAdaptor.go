package authentication

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// EmailAndPasswordAuthenticatorRESTAdaptor exposes email/password authentication over a REST API.
type EmailAndPasswordAuthenticatorRESTAdaptor struct {
	authenticator EmailAndPasswordAuthenticatorService
}

func NewEmailAndPasswordAuthenticatorRESTAdaptor(
	authenticator EmailAndPasswordAuthenticatorService,
) *EmailAndPasswordAuthenticatorRESTAdaptor {
	return &EmailAndPasswordAuthenticatorRESTAdaptor{
		authenticator: authenticator,
	}
}

// EmailAndPasswordLoginRESTRequest is the expected JSON body for email/password login.
type EmailAndPasswordLoginRESTRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// EmailAndPasswordLoginRESTResponse is the JSON response after a successful login.
type EmailAndPasswordLoginRESTResponse struct {
	Token  string `json:"token"`
	UserID string `json:"userID"`
	Email  string `json:"email"`
}

// Login handles POST requests to authenticate a user with email and password.
func (a *EmailAndPasswordAuthenticatorRESTAdaptor) Login(w http.ResponseWriter, r *http.Request) {
	var request EmailAndPasswordLoginRESTRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Ctx(r.Context()).Error().Err(err).Msg("failed to decode login request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	resp, err := a.authenticator.AuthenticateWithEmailAndPassword(r.Context(), EmailAndPasswordAuthRequest{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		log.Ctx(r.Context()).Error().Err(err).Msg("failed to authenticate with email and password")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(EmailAndPasswordLoginRESTResponse{
		Token:  resp.Token,
		UserID: resp.UserID,
		Email:  resp.Email,
	})
}
