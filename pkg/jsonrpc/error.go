package jsonrpc

import (
	"encoding/json"
	"net/http"
)

// JSON-RPC 2.0 error codes. -32700/-32602/-32603 are spec codes (parse error,
// invalid params, internal error); the -3200x codes are application-defined
// (the spec reserves -32000 to -32099 for implementation-defined server
// errors). These are the single source of truth for both the middleware
// (WriteError) and the codec error mapper (MapError) so every error envelope —
// whoever writes it — uses the same code for the same condition.
const (
	CodeParseError          = -32700
	CodeInvalidParams       = -32602
	CodeInternal            = -32603
	CodeUnauthorized        = -32001
	CodeForbidden           = -32002
	CodeNotFound            = -32003
	CodeAlreadyExists       = -32004
	CodeInsufficientBalance = -32005
	CodeTooManyRequests     = -32006
)

// RequestEnvelope is the minimal slice of a JSON-RPC request callers need to
// route or authorize: the method being called and the id to echo back on an
// error.
type RequestEnvelope struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
}

// WriteError writes a JSON-RPC 2.0 error response echoing the request id. The
// reason is a stable machine-readable token surfaced under error.data, matching
// the shape MapError produces for handler errors so a client sees one
// consistent envelope regardless of which layer rejected the request.
// JSON-RPC errors are transported with HTTP 200; the error lives in the body.
func WriteError(w http.ResponseWriter, id json.RawMessage, code int, message, reason string) {
	WriteErrorStatus(w, http.StatusOK, id, code, message, reason)
}

// WriteErrorStatus is WriteError with an explicit HTTP status. Almost every
// JSON-RPC error rides on HTTP 200 (the error is in the body), but a few
// transport-level rejections — notably rate limiting — set a status the body
// also reflects so load balancers and generic clients can act on it (e.g. 429).
func WriteErrorStatus(w http.ResponseWriter, status int, id json.RawMessage, code int, message, reason string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}

	response := struct {
		Version string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   struct {
			Code    int            `json:"code"`
			Message string         `json:"message"`
			Data    map[string]any `json:"data,omitempty"`
		} `json:"error"`
	}{Version: "2.0", ID: id}
	response.Error.Code = code
	response.Error.Message = message
	if reason != "" {
		response.Error.Data = map[string]any{"reason": reason}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
