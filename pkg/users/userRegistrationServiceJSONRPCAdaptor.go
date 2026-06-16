package users

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// UserRegistrationServiceJSONRPCAdaptor exposes UserRegistrationService over
// JSON-RPC. Its Register method is public — a caller has no token before they
// sign up — and hands back an access token so they are logged in straight away.
type UserRegistrationServiceJSONRPCAdaptor struct {
	registrationService UserRegistrationService
}

func NewUserRegistrationServiceJSONRPCAdaptor(
	registrationService UserRegistrationService,
) *UserRegistrationServiceJSONRPCAdaptor {
	return &UserRegistrationServiceJSONRPCAdaptor{
		registrationService: registrationService,
	}
}

func (a *UserRegistrationServiceJSONRPCAdaptor) Name() string {
	return "UserRegistrationService"
}

type RegisterUserRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	AccountName string `json:"accountName"`
}

type RegisterUserResponse struct {
	Token     string `json:"token"`
	UserID    string `json:"userID"`
	AccountID string `json:"accountID"`
	Email     string `json:"email"`
}

func (a *UserRegistrationServiceJSONRPCAdaptor) Register(r *http.Request, request *RegisterUserRequest, response *RegisterUserResponse) error {
	registerResp, err := a.registrationService.Register(r.Context(), RegisterRequest{
		Email:       request.Email,
		Password:    request.Password,
		Name:        request.Name,
		AccountName: request.AccountName,
	})
	if err != nil {
		log.Ctx(r.Context()).Error().Err(err).Msg("failed to register user")
		switch {
		case errors.Is(err, errs.ErrAlreadyExists):
			return errs.WithMessage(errs.ErrAlreadyExists, "an account with this email already exists")
		case errors.Is(err, errs.ErrInvalidArgument):
			return err
		default:
			return errs.WithMessage(errs.ErrInternal, "could not register user")
		}
	}

	response.Token = registerResp.Token
	response.UserID = registerResp.UserID
	response.AccountID = registerResp.AccountID
	response.Email = registerResp.Email
	return nil
}
