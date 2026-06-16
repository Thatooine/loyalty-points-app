package authentication

import (
	"context"

	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

// LogoutServiceMock is a test double for authentication.LogoutService. Set
// LogoutFn to control the behaviour of Logout.
type LogoutServiceMock struct {
	LogoutFn func(ctx context.Context, request pkgAuth.LogoutRequest) (*pkgAuth.LogoutResponse, error)
}

// Logout delegates to LogoutFn.
func (m *LogoutServiceMock) Logout(ctx context.Context, request pkgAuth.LogoutRequest) (*pkgAuth.LogoutResponse, error) {
	return m.LogoutFn(ctx, request)
}
