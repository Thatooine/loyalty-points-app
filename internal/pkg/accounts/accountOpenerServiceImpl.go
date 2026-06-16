package accounts

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
)

// defaultAccountName is the wallet name used when the request leaves it blank.
// It mirrors the default the registration flow applies to a new user's first
// account.
const defaultAccountName = "Primary Wallet"

// AccountOpenerServiceImpl opens a new account by delegating the persistence to
// AccountRepository.Create. The domain rules live here, not in the repository:
// it defaults a blank name and forces the new account's owner to the caller,
// keeping the repository policy-free.
type AccountOpenerServiceImpl struct {
	accounts pkgAccounts.AccountRepository
}

func NewAccountOpenerServiceImpl(accounts pkgAccounts.AccountRepository) *AccountOpenerServiceImpl {
	return &AccountOpenerServiceImpl{accounts: accounts}
}

func (s *AccountOpenerServiceImpl) OpenAccount(ctx context.Context, request pkgAccounts.OpenAccountRequest) (*pkgAccounts.OpenAccountResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for OpenAccount: %w", err)
	}

	name := request.Name
	if name == "" {
		name = defaultAccountName
	}

	// Owner is pinned to the caller (request.UserID, already resolved from the
	// login claim) and the balance starts at zero. Create assigns the ID.
	created, err := s.accounts.Create(ctx, pkgAccounts.CreateAccountRequest{
		Account: pkgAccounts.Account{
			OwnerID:   request.UserID,
			Name:      name,
			Balance:   0,
			CreatedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not open account: %w", err)
	}

	return &pkgAccounts.OpenAccountResponse{Account: created.Account}, nil
}
