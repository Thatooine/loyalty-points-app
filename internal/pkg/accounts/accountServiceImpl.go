package accounts

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
)

type AccountServiceImpl struct {
	accounts pkgAccounts.AccountRepository
}

func NewAccountServiceImpl(accounts pkgAccounts.AccountRepository) *AccountServiceImpl {
	return &AccountServiceImpl{accounts: accounts}
}

func (s *AccountServiceImpl) GetAccountByID(ctx context.Context, request pkgAccounts.ReadAccountRequest) (*pkgAccounts.ReadAccountResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetAccountByID: %w", err)
	}

	resp, err := s.accounts.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{
		AccountID: request.AccountID,
		UserID:    request.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	return &pkgAccounts.ReadAccountResponse{Account: resp.Account}, nil
}

func (s *AccountServiceImpl) GetAccountBalance(ctx context.Context, request pkgAccounts.ReadAccountBalanceRequest) (*pkgAccounts.ReadAccountBalanceResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetAccountBalance: %w", err)
	}

	resp, err := s.accounts.GetAccountBalance(ctx, pkgAccounts.GetAccountBalanceRequest{
		AccountID: request.AccountID,
		UserID:    request.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get account balance: %w", err)
	}

	return &pkgAccounts.ReadAccountBalanceResponse{Balance: resp.Balance}, nil
}

func (s *AccountServiceImpl) UpdateAccountName(ctx context.Context, request pkgAccounts.RenameAccountRequest) (*pkgAccounts.RenameAccountResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for UpdateAccountName: %w", err)
	}

	resp, err := s.accounts.UpdateAccountName(ctx, pkgAccounts.UpdateAccountNameRequest{
		AccountID: request.AccountID,
		Name:      request.Name,
		UserID:    request.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not update account name: %w", err)
	}

	return &pkgAccounts.RenameAccountResponse{Account: resp.Account}, nil
}

func (s *AccountServiceImpl) UpdateAccountBalance(ctx context.Context, request pkgAccounts.AdjustAccountBalanceRequest) (*pkgAccounts.AdjustAccountBalanceResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for UpdateAccountBalance: %w", err)
	}

	resp, err := s.accounts.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{
		AccountID: request.AccountID,
		Delta:     request.Delta,
		UserID:    request.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not update account balance: %w", err)
	}

	return &pkgAccounts.AdjustAccountBalanceResponse{Balance: resp.Balance}, nil
}
