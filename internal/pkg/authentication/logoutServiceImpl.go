package authentication

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// LogoutServiceImpl implements authentication.LogoutService by bumping the
// user's token_version through the user repository. One increment invalidates
// every access token the user currently holds (see LogoutService docs).
type LogoutServiceImpl struct {
	userRepository pkgUsers.UserRepository
}

func NewLogoutServiceImpl(userRepository pkgUsers.UserRepository) *LogoutServiceImpl {
	return &LogoutServiceImpl{userRepository: userRepository}
}

func (s *LogoutServiceImpl) Logout(ctx context.Context, request pkgAuth.LogoutRequest) (*pkgAuth.LogoutResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for Logout: %w", err)
	}

	resp, err := s.userRepository.IncrementTokenVersion(ctx, pkgUsers.IncrementTokenVersionRequest{UserID: request.UserID})
	if err != nil {
		return nil, fmt.Errorf("could not revoke tokens: %w", err)
	}

	return &pkgAuth.LogoutResponse{TokenVersion: resp.TokenVersion}, nil
}
