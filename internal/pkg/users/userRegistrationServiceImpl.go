package users

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// registrationTokenTTL is how long the access token issued at registration
// remains valid (mirrors the email/password auth service).
const registrationTokenTTL = 24 * time.Hour

// defaultAccountName is the wallet name used when the request leaves it blank.
const defaultAccountName = "Primary Wallet"

// UserRegistrationServiceImpl onboards a new principal by composing the user
// and account repositories inside one unit of work, then issuing an access
// token. Creating the identity and opening the wallet either both commit or
// neither does, so a failed account-open never leaves a credential-only user
// behind.
type UserRegistrationServiceImpl struct {
	txManager          pkgSQL.TxManager
	users              pkgUsers.UserRepository
	accounts           pkgAccounts.AccountRepository
	accessTokenService pkgAuth.AccessTokenService
}

func NewUserRegistrationServiceImpl(
	txManager pkgSQL.TxManager,
	users pkgUsers.UserRepository,
	accounts pkgAccounts.AccountRepository,
	accessTokenService pkgAuth.AccessTokenService,
) *UserRegistrationServiceImpl {
	return &UserRegistrationServiceImpl{
		txManager:          txManager,
		users:              users,
		accounts:           accounts,
		accessTokenService: accessTokenService,
	}
}

func (s *UserRegistrationServiceImpl) Register(ctx context.Context, request pkgUsers.RegisterRequest) (*pkgUsers.RegisterResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for Register: %w", err)
	}

	// Hash the password before it touches the database — the plaintext never
	// leaves this method.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("could not hash password: %w", err)
	}

	accountName := request.AccountName
	if accountName == "" {
		accountName = defaultAccountName
	}

	user := pkgUsers.User{
		ID:           uuid.NewString(),
		Email:        request.Email,
		PasswordHash: string(passwordHash),
		Role:         pkgUsers.RoleMember,
		CreatedAt:    time.Now().UTC(),
	}

	var accountID string

	// One unit of work: the user row and their first account commit together
	// or roll back together. The unique constraint on email is the duplicate
	// guard — we insert first rather than check-then-insert, consistent with
	// the wallet idempotency stance.
	err = s.txManager.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := s.users.Create(ctx, pkgUsers.CreateUserRequest{User: user}); err != nil {
			return err
		}

		created, err := s.accounts.Create(ctx, pkgAccounts.CreateAccountRequest{
			Account: pkgAccounts.Account{
				UserID:    user.ID,
				Name:      accountName,
				Balance:   0,
				CreatedAt: time.Now().UTC(),
			},
		})
		if err != nil {
			return err
		}
		accountID = created.Account.ID
		return nil
	})
	if err != nil {
		log.Ctx(ctx).Warn().Str("email", request.Email).Err(err).Msg("registration failed")
		return nil, fmt.Errorf("could not register user: %w", err)
	}

	// Token issuance is not a database write, so it runs after the unit of
	// work has committed.
	tokenResp, err := s.accessTokenService.IssueAccessToken(ctx, pkgAuth.IssueAccessTokenRequest{
		LoginClaim: pkgAuth.LoginClaim{
			UserID:         user.ID,
			Email:          user.Email,
			Role:           user.Role,
			ExpirationTime: time.Now().Add(registrationTokenTTL).Unix(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not issue access token: %w", err)
	}

	return &pkgUsers.RegisterResponse{
		Token:     tokenResp.AccessToken,
		UserID:    user.ID,
		AccountID: accountID,
		Email:     user.Email,
	}, nil
}
