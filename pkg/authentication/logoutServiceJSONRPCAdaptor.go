package authentication

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// The principal is taken from the verified claim, never the wire, so a caller
// can only log themselves out.
type LogoutServiceJSONRPCAdaptor struct {
	logoutService LogoutService
}

func NewLogoutServiceJSONRPCAdaptor(logoutService LogoutService) *LogoutServiceJSONRPCAdaptor {
	return &LogoutServiceJSONRPCAdaptor{logoutService: logoutService}
}

func (a *LogoutServiceJSONRPCAdaptor) Name() string {
	return "Session"
}

type LogoutJSONRPCRequest struct{}

type LogoutJSONRPCResponse struct {
	OK bool `json:"ok"`
}

func (a *LogoutServiceJSONRPCAdaptor) Logout(r *http.Request, _ *LogoutJSONRPCRequest, result *LogoutJSONRPCResponse) error {
	ctx := r.Context()

	claim, ok := LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("session: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	if _, err := a.logoutService.Logout(ctx, LogoutRequest{UserID: claim.UserID}); err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("userID", claim.UserID).Msg("session: logout failed")
		return errs.WithMessage(errs.ErrInternal, "could not log out")
	}

	result.OK = true
	return nil
}
