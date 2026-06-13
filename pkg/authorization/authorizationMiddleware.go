package authorization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

// JSON-RPC 2.0 error codes. -32700 is the spec's parse-error code; the
// 32001/32002 codes are application-defined (the spec reserves -32000 to
// -32099 for implementation-defined server errors).
const (
	codeParseError   = -32700
	codeUnauthorized = -32001
	codeForbidden    = -32002
)

// rpcEnvelope is the minimal slice of a JSON-RPC request we need: the method to
// authorize and the id to echo back on an error.
type rpcEnvelope struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
}

// NewAuthorizationMiddleware returns a gorilla/mux-compatible middleware that
// enforces role-based access to JSON-RPC methods. It assumes an upstream
// middleware (authentication.NewAuthMiddleware) has already validated the
// token and placed the LoginClaim in the request context; the role is read
// from that claim. A caller lacking permission receives a JSON-RPC error
// envelope rather than reaching the handler.
func NewAuthorizationMiddleware(perms *Permissions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Read the body to learn the method, then restore it so the
			// JSON-RPC codec downstream can read it again.
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).Msg("authorization: could not read request body")
				writeRPCError(w, nil, codeParseError, "could not read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			var envelope rpcEnvelope
			if err := json.Unmarshal(body, &envelope); err != nil || envelope.Method == "" {
				log.Ctx(ctx).Warn().Err(err).Msg("authorization: could not parse JSON-RPC method")
				writeRPCError(w, envelope.ID, codeParseError, "could not parse JSON-RPC request")
				return
			}

			// Identify the caller from the verified token claim. Behind the
			// auth middleware this is always present; the guard is defensive.
			claim, ok := authentication.LoginClaimFromContext(ctx)
			if !ok {
				log.Ctx(ctx).Warn().Str("method", envelope.Method).Msg("authorization: no login claim in context")
				writeRPCError(w, envelope.ID, codeUnauthorized, "unauthorized")
				return
			}

			if !perms.Can(claim.Role, envelope.Method) {
				log.Ctx(ctx).Warn().
					Str("userID", claim.UserID).
					Str("role", string(claim.Role)).
					Str("method", envelope.Method).
					Msg("authorization: role not permitted to call method")
				writeRPCError(w, envelope.ID, codeForbidden,
					fmt.Sprintf("role %q may not call %q", claim.Role, envelope.Method))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeRPCError writes a JSON-RPC 2.0 error response echoing the request id.
// JSON-RPC errors are transported with HTTP 200; the error lives in the body.
func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}

	response := struct {
		Version string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{Version: "2.0", ID: id}
	response.Error.Code = code
	response.Error.Message = message

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
