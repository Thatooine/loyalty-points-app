package accounts

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

// UserID is taken from the verified claim, never the wire, so every lookup is
// scoped to the caller's own accounts.
type AccountServiceJSONRPCAdaptor struct {
	accounts AccountService
}

func NewAccountServiceJSONRPCAdaptor(accounts AccountService) *AccountServiceJSONRPCAdaptor {
	return &AccountServiceJSONRPCAdaptor{accounts: accounts}
}

func (a *AccountServiceJSONRPCAdaptor) Name() string {
	return "AccountService"
}

type GetByIDParams struct {
	AccountID string `json:"account_id"`
}

type AccountResult struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

type BalanceResult struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
}

type FetchMyAccountsParams struct{}

type AccountsResult struct {
	Accounts []AccountResult `json:"accounts"`
}

// FetchMyAccounts lists every account the caller directly owns. UserID is taken
// from the verified claim, so the listing is always scoped to the caller.
func (a *AccountServiceJSONRPCAdaptor) FetchMyAccounts(r *http.Request, params *FetchMyAccountsParams, result *AccountsResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("account: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.accounts.FetchMyAccounts(ctx, FetchMyAccountsRequest{
		UserID: claim.UserID,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("userID", claim.UserID).Msg("account: fetch my accounts failed")
		if errors.Is(err, errs.ErrInvalidArgument) {
			return err
		}
		return errs.WithMessage(errs.ErrInternal, "could not retrieve accounts")
	}

	result.Accounts = make([]AccountResult, 0, len(resp.Accounts))
	for _, account := range resp.Accounts {
		result.Accounts = append(result.Accounts, AccountResult{
			ID:        account.ID,
			UserID:    account.OwnerID,
			Name:      account.Name,
			Balance:   account.Balance,
			CreatedAt: account.CreatedAt,
		})
	}
	return nil
}

func (a *AccountServiceJSONRPCAdaptor) GetAccountByID(r *http.Request, params *GetByIDParams, result *AccountResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("account: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.accounts.GetAccountByID(ctx, ReadAccountRequest{
		AccountID: params.AccountID,
		UserID:    claim.UserID,
	})
	if err != nil {
		return notFoundOrInternal(ctx, err, params.AccountID)
	}

	result.ID = resp.Account.ID
	result.UserID = resp.Account.OwnerID
	result.Name = resp.Account.Name
	result.Balance = resp.Account.Balance
	result.CreatedAt = resp.Account.CreatedAt
	return nil
}

func (a *AccountServiceJSONRPCAdaptor) GetAccountBalance(r *http.Request, params *GetByIDParams, result *BalanceResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("account: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.accounts.GetAccountBalance(ctx, ReadAccountBalanceRequest{
		AccountID: params.AccountID,
		UserID:    claim.UserID,
	})
	if err != nil {
		return notFoundOrInternal(ctx, err, params.AccountID)
	}

	result.AccountID = params.AccountID
	result.Balance = resp.Balance
	return nil
}

type UpdateAccountNameParams struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
}

func (a *AccountServiceJSONRPCAdaptor) UpdateAccountName(r *http.Request, params *UpdateAccountNameParams, result *AccountResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("account: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.accounts.UpdateAccountName(ctx, RenameAccountRequest{
		AccountID: params.AccountID,
		Name:      params.Name,
		UserID:    claim.UserID,
	})
	if err != nil {
		return notFoundOrInternal(ctx, err, params.AccountID)
	}

	result.ID = resp.Account.ID
	result.UserID = resp.Account.OwnerID
	result.Name = resp.Account.Name
	result.Balance = resp.Account.Balance
	result.CreatedAt = resp.Account.CreatedAt
	return nil
}

type UpdateAccountBalanceParams struct {
	AccountID string `json:"account_id"`
	Delta     int64  `json:"delta"`
}

// Explicit admin check is defence in depth: the policy already gates this, but
// it must not drift if the permission map changes.
func (a *AccountServiceJSONRPCAdaptor) UpdateAccountBalance(r *http.Request, params *UpdateAccountBalanceParams, result *BalanceResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("account: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}
	if claim.Role != users.RoleAdmin {
		log.Ctx(ctx).Warn().Str("userID", claim.UserID).Msg("account: non-admin attempted raw balance update")
		return errs.WithMessage(errs.ErrForbidden, "balance adjustment is admin-only")
	}

	resp, err := a.accounts.UpdateAccountBalance(ctx, AdjustAccountBalanceRequest{
		AccountID: params.AccountID,
		Delta:     params.Delta,
		UserID:    claim.UserID,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("accountID", params.AccountID).Msg("account: balance update failed")
		switch {
		case errors.Is(err, errs.ErrInsufficientBalance):
			return errs.WithMessage(errs.ErrInsufficientBalance, "insufficient balance")
		case errors.Is(err, errs.ErrNotFound):
			return errs.WithMessage(errs.ErrNotFound, "account not found")
		case errors.Is(err, errs.ErrInvalidArgument):
			return err
		default:
			return errs.WithMessage(errs.ErrInternal, "could not update account balance")
		}
	}

	result.AccountID = params.AccountID
	result.Balance = resp.Balance
	return nil
}

func notFoundOrInternal(ctx context.Context, err error, accountID string) error {
	log.Ctx(ctx).Warn().Err(err).Str("accountID", accountID).Msg("account: lookup failed")
	switch {
	case errors.Is(err, errs.ErrNotFound):
		return errs.WithMessage(errs.ErrNotFound, "account not found")
	case errors.Is(err, errs.ErrInvalidArgument):
		return err
	default:
		return errs.WithMessage(errs.ErrInternal, "could not retrieve account")
	}
}
