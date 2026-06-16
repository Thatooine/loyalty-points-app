package authentication

import (
	"context"

	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

// AccessTokenServiceMock is a test double for AccessTokenService. Set
// IssueAccessTokenFn and ValidateAccessTokenFn to control the behaviour of the
// corresponding methods.
type AccessTokenServiceMock struct {
	IssueAccessTokenFn    func(ctx context.Context, request pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error)
	ValidateAccessTokenFn func(ctx context.Context, request pkgAuth.ValidateAccessTokenRequest) (*pkgAuth.ValidateAccessTokenResponse, error)
}

func (m *AccessTokenServiceMock) IssueAccessToken(ctx context.Context, request pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error) {
	return m.IssueAccessTokenFn(ctx, request)
}

func (m *AccessTokenServiceMock) ValidateAccessToken(ctx context.Context, request pkgAuth.ValidateAccessTokenRequest) (*pkgAuth.ValidateAccessTokenResponse, error) {
	return m.ValidateAccessTokenFn(ctx, request)
}
