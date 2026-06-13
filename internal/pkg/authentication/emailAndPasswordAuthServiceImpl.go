package authentication

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// accessTokenTTL is how long an issued access token remains valid.
const accessTokenTTL = 24 * time.Hour

type EmailAndPasswordAuthServiceImpl struct {
	users              pkgUsers.UserRepository
	accessTokenService pkgAuth.AccessTokenService
}

func NewEmailAndPasswordAuthServiceImpl(
	users pkgUsers.UserRepository,
	accessTokenService pkgAuth.AccessTokenService,
) *EmailAndPasswordAuthServiceImpl {
	return &EmailAndPasswordAuthServiceImpl{
		users:              users,
		accessTokenService: accessTokenService,
	}
}

// A missing user and a wrong password both yield errs.ErrUnauthorized so the
// caller cannot tell which emails are registered.
func (s *EmailAndPasswordAuthServiceImpl) Authenticate(ctx context.Context, request pkgAuth.EmailAndPasswordAuthRequest) (*pkgAuth.EmailAndPasswordAuthResponse, error) {
	// 1. retrieve the user entity by email
	userResp, err := s.users.GetByEmail(ctx, pkgUsers.GetUserByEmailRequest{Email: request.Email})
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			log.Ctx(ctx).Warn().Str("email", request.Email).Msg("authentication failed: no user for email")
			return nil, errs.ErrUnauthorized
		}
		return nil, fmt.Errorf("could not retrieve user by email: %w", err)
	}
	user := userResp.User

	// 2 & 3. compare the password against the stored bcrypt hash. bcrypt embeds
	// the salt in the hash, so the comparison hashes the candidate internally —
	// there is no separate hash-then-equals step.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)); err != nil {
		log.Ctx(ctx).Warn().Str("userID", user.ID).Msg("authentication failed: password mismatch")
		return nil, errs.ErrUnauthorized
	}

	// 4. password is correct — issue a token from a claim identifying the user.
	tokenResp, err := s.accessTokenService.IssueAccessToken(
		ctx,
		pkgAuth.IssueAccessTokenRequest{
			LoginClaim: pkgAuth.LoginClaim{
				UserID:         user.ID,
				Email:          user.Email,
				ExpirationTime: time.Now().Add(accessTokenTTL).Unix(),
			},
		})
	if err != nil {
		return nil, fmt.Errorf("could not issue access token: %w", err)
	}

	// 5. return the token, user id and email
	return &pkgAuth.EmailAndPasswordAuthResponse{
		Token:  tokenResp.AccessToken,
		UserID: user.ID,
		Email:  user.Email,
	}, nil
}
