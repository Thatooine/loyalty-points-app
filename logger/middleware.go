package logger

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

// Middleware injects the global logger into each request's context
// so that log.Ctx(r.Context()) works throughout the handler chain.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := log.Logger.WithContext(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
