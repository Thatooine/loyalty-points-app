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
//     then checks the caller's permissions against the method (authorization).
//
// A caller who fails either check receives a JSON-RPC error envelope rather
// than reaching the handler. On success the verified LoginClaim is placed in
// the request context. Method gating here is all-or-nothing; how broadly the
// caller may act on the data (own vs all) is a separate decision resolved on
// demand by the data layer from the claim's permissions (see IsGranted).
func NewAuthorizationMiddleware(accessTokenService authentication.AccessTokenValidator, policy *Policy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Read the body to learn the method, then restore it so the
			// JSON-RPC codec downstream can read it again.
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).Msg("authorization: could not read request body")
				jsonrpc.WriteError(w, nil, jsonrpc.CodeParseError, "could not read request body", "parse")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			var envelope jsonrpc.RequestEnvelope
			if err := json.Unmarshal(body, &envelope); err != nil || envelope.Method == "" {
				log.Ctx(ctx).Warn().Err(err).Msg("authorization: could not parse JSON-RPC method")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeParseError, "could not parse JSON-RPC request", "parse")
				return
			}

			// Public methods need no token — pass straight through.
			if policy.IsPublic(envelope.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Authenticate: a protected method requires a valid token.
			token := authentication.ExtractToken(r)
			if token == "" {
				log.Ctx(ctx).Warn().Str("method", envelope.Method).Msg("authorization: no access token")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeUnauthorized, "unauthorized", "unauthorized")
				return
			}
			tokenResp, err := accessTokenService.ValidateAccessToken(ctx, authentication.ValidateAccessTokenRequest{AccessToken: token})
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).Str("method", envelope.Method).Msg("authorization: token validation failed")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeUnauthorized, "unauthorized", "unauthorized")
				return
			}
			claim := tokenResp.LoginClaim

			// Authorize: the caller must hold a permission the method accepts.
			// This gate is all-or-nothing — it does not resolve a scope; the data
			// layer enforces own-vs-all later via IsGranted.
			if !policy.Authorize(claim.Permissions, envelope.Method) {
				log.Ctx(ctx).Warn().
					Str("userID", claim.UserID).
					Strs("permissions", claim.Permissions).
					Str("method", envelope.Method).
					Msg("authorization: caller lacks a permission for method")
				jsonrpc.WriteError(w, envelope.ID, jsonrpc.CodeForbidden,
					fmt.Sprintf("not permitted to call %q", envelope.Method), "forbidden")
				return
			}

			// Hand the verified claim to downstream handlers; ownership scope is
			// derived from its permissions on demand (see IsGranted).
			ctx = authentication.ContextWithLoginClaim(ctx, claim)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
