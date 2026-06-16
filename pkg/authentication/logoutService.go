package authentication

import (
	"context"
	"fmt"
)

// LogoutService revokes a user's outstanding access tokens. It does so by
// bumping the user's token_version (session epoch): every token issued before
// the bump carries a now-stale version and is rejected by ValidateAccessToken.
//
// Because the version is a single per-user value, logout is "log out
// everywhere" — it invalidates the caller's tokens on every device at once.
// Per-device logout would require per-token tracking, which this does not do.
type LogoutService interface {
	Logout(ctx context.Context, request LogoutRequest) (*LogoutResponse, error)
}

type LogoutRequest struct {
	// Taken from the verified login claim by the adaptor, never from client
	// input, so a caller can only log themselves out.
	UserID string
}

type LogoutResponse struct {
	TokenVersion int64
}

func (r *LogoutRequest) Validate() error {
	if r.UserID == "" {
		return fmt.Errorf("validation failed: UserID is required")
	}
	return nil
}
