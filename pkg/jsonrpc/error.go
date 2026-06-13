package jsonrpc

import (
	"encoding/json"
	"net/http"
)

// JSON-RPC 2.0 error codes. -32700 is the spec's parse-error code; the
// 32001/32002 codes are application-defined (the spec reserves -32000 to
// -32099 for implementation-defined server errors).
const (
	CodeParseError   = -32700
	CodeUnauthorized = -32001
	CodeForbidden    = -32002
)

// RequestEnvelope is the minimal slice of a JSON-RPC request callers need to
// route or authorize: the method being called and the id to echo back on an
// error.
type RequestEnvelope struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
}

// WriteError writes a JSON-RPC 2.0 error response echoing the request id.
// JSON-RPC errors are transported with HTTP 200; the error lives in the body.
func WriteError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
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
