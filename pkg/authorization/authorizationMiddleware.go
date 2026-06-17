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

// Method gating here is all-or-nothing; own-vs-all scope is resolved later by
// the data layer from the claim's permissions (see IsGranted).
func NewAuthorizationMiddleware(accessTokenService authentication.AccessTokenValidator, policy *Policy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Restore the body after reading so the JSON-RPC codec can read it again.
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

			if policy.IsPublic(envelope.Method) {
				next.ServeHTTP(w, r)
				return
			}

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

			ctx = authentication.ContextWithLoginClaim(ctx, claim)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
