package authentication

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

// LogoutServiceJSONRPCAdaptor exposes LogoutService over JSON-RPC as
// "Session.Logout". Like the other protected adaptors, the acting principal is
// taken from the verified login claim placed on the context by the
// authorization middleware — never from the request body — so a caller can only
// log themselves out.
type LogoutServiceJSONRPCAdaptor struct {
	logoutService LogoutService
}

func NewLogoutServiceJSONRPCAdaptor(logoutService LogoutService) *LogoutServiceJSONRPCAdaptor {
	return &LogoutServiceJSONRPCAdaptor{logoutService: logoutService}
}

func (a *LogoutServiceJSONRPCAdaptor) Name() string {
	return "Session"
}

// LogoutJSONRPCRequest is the wire request. It carries no fields: the principal
// is resolved from the token, so the client sends nothing.
type LogoutJSONRPCRequest struct{}

// LogoutJSONRPCResponse is the wire response.
type LogoutJSONRPCResponse struct {
	// OK is always true on success; it gives the client a non-empty body to
	// assert against.
	OK bool `json:"ok"`
}

func (a *LogoutServiceJSONRPCAdaptor) Logout(r *http.Request, _ *LogoutJSONRPCRequest, result *LogoutJSONRPCResponse) error {
	ctx := r.Context()

	claim, ok := LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("session: no login claim in context for protected method")
		return errors.New("unauthorized")
	}

	if _, err := a.logoutService.Logout(ctx, LogoutRequest{UserID: claim.UserID}); err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("userID", claim.UserID).Msg("session: logout failed")
		return errors.New("could not log out")
	}

	result.OK = true
	return nil
}
