package authentication

import (
	"context"
	"fmt"
)

// LogoutService revokes a user's outstanding access tokens by bumping their
// token_version. The version is a single per-user value, so this is necessarily
// "log out everywhere"; per-device logout would require per-token tracking.
type LogoutService interface {
	Logout(ctx context.Context, request LogoutRequest) (*LogoutResponse, error)
}

type LogoutRequest struct {
	// UserID is taken from the verified login claim, never from client input,
	// so a caller can only log themselves out.
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
