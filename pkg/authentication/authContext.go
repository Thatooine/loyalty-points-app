package authentication

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const loginClaimContextKey contextKey = "loginClaim"

// ContextWithLoginClaim returns a copy of ctx carrying the given LoginClaim,
// keyed so that LoginClaimFromContext can retrieve it.
func ContextWithLoginClaim(ctx context.Context, claim LoginClaim) context.Context {
	return context.WithValue(ctx, loginClaimContextKey, claim)
}

// LoginClaimFromContext retrieves the LoginClaim stored by the auth middleware.
// Returns the claim and true if present, or a zero value and false otherwise.
func LoginClaimFromContext(ctx context.Context) (LoginClaim, bool) {
	claim, ok := ctx.Value(loginClaimContextKey).(LoginClaim)
	return claim, ok
}

// ExtractToken looks for an access token first in the Authorization header
// (expecting "Bearer <token>"), then in a cookie named "access_token".
func ExtractToken(r *http.Request) string {
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if token, found := strings.CutPrefix(authHeader, "Bearer "); found {
			return token
		}
	}

	return ""
}
