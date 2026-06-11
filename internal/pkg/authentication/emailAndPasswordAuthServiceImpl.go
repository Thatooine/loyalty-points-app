package authentication

import (
	"context"

	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

type EmailAndPasswordAuthServiceImpl struct {
	accessTokenService pkgAuth.AccessTokenService
}

func NewEmailAndPasswordAuthServiceImpl(
	accessTokenService pkgAuth.AccessTokenService,
) *EmailAndPasswordAuthServiceImpl {
	return &EmailAndPasswordAuthServiceImpl{
		accessTokenService: accessTokenService,
	}
}

func (s *EmailAndPasswordAuthServiceImpl) Authenticate(ctx context.Context, request pkgAuth.EmailAndPasswordAuthRequest) (*pkgAuth.EmailAndPasswordAuthResponse, error) {

	return &pkgAuth.EmailAndPasswordAuthResponse{}, nil
}
