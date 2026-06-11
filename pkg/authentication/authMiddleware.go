package authentication

import (
	"context"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

type contextKey string

const loginClaimContextKey contextKey = "loginClaim"

// NewAuthMiddleware returns a gorilla/mux-compatible middleware that checks for
// an access token in the Authorization header ("Bearer <token>") or in a cookie
// named "access_token". If a valid token is found, the decoded LoginClaim is
// stored in the request context and the next handler is called. Otherwise it
// responds with 401 Unauthorized.
func NewAuthMiddleware(accessTokenService AccessTokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				log.Ctx(r.Context()).Warn().Msg("no access token found in request")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			resp, err := accessTokenService.ValidateAccessToken(r.Context(), ValidateAccessTokenRequest{
				AccessToken: token,
			})
			if err != nil {
				log.Ctx(r.Context()).Warn().Err(err).Msg("access token validation failed")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), loginClaimContextKey, resp.LoginClaim)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LoginClaimFromContext retrieves the LoginClaim stored by the auth middleware.
// Returns the claim and true if present, or a zero value and false otherwise.
func LoginClaimFromContext(ctx context.Context) (LoginClaim, bool) {
	claim, ok := ctx.Value(loginClaimContextKey).(LoginClaim)
	return claim, ok
}

// extractToken looks for an access token first in the Authorization header
// (expecting "Bearer <token>"), then in a cookie named "access_token".
func extractToken(r *http.Request) string {
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if token, found := strings.CutPrefix(authHeader, "Bearer "); found {
			return token
		}
	}

	if cookie, err := r.Cookie("access_token"); err == nil {
		return cookie.Value
	}

	return ""
}
