package authentication

import (
	"context"

	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

type LogoutServiceMock struct {
	LogoutFn func(ctx context.Context, request pkgAuth.LogoutRequest) (*pkgAuth.LogoutResponse, error)
}

func (m *LogoutServiceMock) Logout(ctx context.Context, request pkgAuth.LogoutRequest) (*pkgAuth.LogoutResponse, error) {
	return m.LogoutFn(ctx, request)
}
