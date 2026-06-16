package logger

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// requestIDHeader is the header used to read an inbound correlation id and echo
// the one in effect back to the caller.
const requestIDHeader = "X-Request-ID"

// Middleware injects a request-scoped logger into each request's context so that
// log.Ctx(r.Context()) works throughout the handler chain. Every line logged
// downstream carries a request_id, letting all logs for one request be
// correlated. It also echoes the id back as X-Request-ID and emits one
// structured access-log line per request (status + duration) when the request
// completes.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, requestID)

		logger := log.Logger.With().Str("request_id", requestID).Logger()
		ctx := logger.WithContext(r.Context())

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(recorder, r.WithContext(ctx))

		logger.Info().
			Int("status", recorder.status).
			Dur("duration_ms", time.Since(start)).
			Str("path", r.URL.Path).
			Msg("request completed")
	})
}

// statusRecorder captures the response status code for the access log while
// delegating writes to the wrapped ResponseWriter.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
