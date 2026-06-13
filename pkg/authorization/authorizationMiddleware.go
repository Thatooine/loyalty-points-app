package authorization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
)

// NewAuthorizationMiddleware returns a gorilla/mux-compatible middleware that
// gates a single JSON-RPC endpoint hosting both public and protected services.
// For each request it reads the called method and:
//   - lets public methods (e.g. login) through untouched, so a caller can
//     obtain a token in the first place;
//   - for every other method, validates the access token (authentication),
//     then checks the caller's role against the method (authorization).
//
// A caller who fails either check receives a JSON-RPC error envelope rather
// than reaching the handler. On success the verified LoginClaim is placed in
// the request context for downstream handlers.
func NewAuthorizationMiddleware(accessTokenService authentication.AccessTokenService, perms *Permissions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Read the body to learn the method, then restore it so the
			// JSON-RPC codec downstream can read it again.
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).Msg("authorization: could not read request body")
				jsonrpc.WriteError(w, nil, jsonrpc.CodeParseError, "could not read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			var envelope jsonrpc.RequestEnvelope
			if err := json.Unmarshal(body, &envelope); err != nil || envelope.Method == "" {
				log.Ctx(ctx).Warn().Err(err).Msg("authorization: could not parse JSON-RPC method")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeParseError, "could not parse JSON-RPC request")
				return
			}

			// Public methods need no token — pass straight through.
			if perms.IsPublic(envelope.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Authenticate: a protected method requires a valid token.
			token := authentication.ExtractToken(r)
			if token == "" {
				log.Ctx(ctx).Warn().Str("method", envelope.Method).Msg("authorization: no access token")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeUnauthorized, "unauthorized")
				return
			}
			tokenResp, err := accessTokenService.ValidateAccessToken(ctx, authentication.ValidateAccessTokenRequest{AccessToken: token})
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).Str("method", envelope.Method).Msg("authorization: token validation failed")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeUnauthorized, "unauthorized")
				return
			}
			claim := tokenResp.LoginClaim

			// Authorize: the caller's role must permit the method.
			if !perms.Can(claim.Role, envelope.Method) {
				log.Ctx(ctx).Warn().
					Str("userID", claim.UserID).
					Str("role", string(claim.Role)).
					Str("method", envelope.Method).
					Msg("authorization: role not permitted to call method")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeForbidden,
					fmt.Sprintf("role %q may not call %q", claim.Role, envelope.Method))
				return
			}

			// Hand the verified claim to downstream handlers.
			ctx = authentication.ContextWithLoginClaim(ctx, claim)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
