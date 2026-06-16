package jsonrpc

import (
	"errors"

	gorillaJSON "github.com/gorilla/rpc/v2/json2"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// Stable machine-readable tokens surfaced under error.data.reason. Clients
// branch on these rather than the human-readable message.
const (
	reasonInvalidArgument   = "invalid_argument"
	reasonUnauthorized      = "unauthorized"
	reasonForbidden         = "forbidden"
	reasonNotFound          = "not_found"
	reasonAlreadyExists     = "already_exists"
	reasonInsufficientFunds = "insufficient_balance"
	reasonInternal          = "internal"
)

// MapError is the codec's error mapper (wired via
// json2.NewCustomCodecWithErrorMapper). It is the one place that translates a
// domain error returned by any handler into a JSON-RPC error with a stable code
// and a machine-readable data.reason. Handlers stay free of transport concerns:
// they return errs sentinels (optionally wrapped with errs.WithMessage for
// contextual phrasing) and this decides the code.
//
// For mapped sentinels the message is err.Error() — the sentinel's own text, or
// the friendly text supplied via errs.WithMessage. The default branch is the
// only one that hides the underlying message, so an unexpected error can never
// leak internals to the client.
func MapError(err error) error {
	if err == nil {
		return nil
	}

	var validation *errs.ValidationError
	switch {
	case errors.As(err, &validation):
		return rpcError(CodeInvalidParams, validation.Error(), reasonInvalidArgument, map[string]any{
			"fields": validation.Reasons,
		})
	case errors.Is(err, errs.ErrInvalidArgument):
		return rpcError(CodeInvalidParams, err.Error(), reasonInvalidArgument, nil)
	case errors.Is(err, errs.ErrUnauthorized):
		return rpcError(CodeUnauthorized, err.Error(), reasonUnauthorized, nil)
	case errors.Is(err, errs.ErrForbidden):
		return rpcError(CodeForbidden, err.Error(), reasonForbidden, nil)
	case errors.Is(err, errs.ErrNotFound):
		return rpcError(CodeNotFound, err.Error(), reasonNotFound, nil)
	case errors.Is(err, errs.ErrAlreadyExists):
		return rpcError(CodeAlreadyExists, err.Error(), reasonAlreadyExists, nil)
	case errors.Is(err, errs.ErrInsufficientBalance):
		return rpcError(CodeInsufficientBalance, err.Error(), reasonInsufficientFunds, nil)
	case errors.Is(err, errs.ErrInternal):
		return rpcError(CodeInternal, err.Error(), reasonInternal, nil)
	default:
		return rpcError(CodeInternal, "internal server error", reasonInternal, nil)
	}
}

// rpcError builds the gorilla json2 error the codec writes verbatim. reason is
// always included under data; extra merges in additional fields (e.g. the
// validation field list).
func rpcError(code int, message, reason string, extra map[string]any) error {
	data := map[string]any{"reason": reason}
	for k, v := range extra {
		data[k] = v
	}
	return &gorillaJSON.Error{
		Code:    gorillaJSON.ErrorCode(code),
		Message: message,
		Data:    data,
	}
}
