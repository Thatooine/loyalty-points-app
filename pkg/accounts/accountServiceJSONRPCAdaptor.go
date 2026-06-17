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

// AccountServiceJSONRPCAdaptor exposes AccountService over JSON-RPC. It is a
// protected service: the authorization middleware places the verified login
// claim in the request context. Every lookup is scoped to the caller's user id
// taken from that claim, so a caller can only read their own accounts —
// requesting another user's account id yields "account not found", with no way
// to tell it apart from a genuinely missing one.
type AccountServiceJSONRPCAdaptor struct {
	accounts AccountService
}

func NewAccountServiceJSONRPCAdaptor(accounts AccountService) *AccountServiceJSONRPCAdaptor {
	return &AccountServiceJSONRPCAdaptor{accounts: accounts}
}

func (a *AccountServiceJSONRPCAdaptor) Name() string {
	return "AccountService"
}

// GetByIDParams is the wire request for GetAccountByID and GetAccountBalance.
type GetByIDParams struct {
	AccountID string `json:"account_id"`
}

// AccountResult is the wire representation of an account.
type AccountResult struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

// BalanceResult is the wire response for GetAccountBalance.
type BalanceResult struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
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

// UpdateAccountNameParams is the wire request for UpdateAccountName.
type UpdateAccountNameParams struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
}

// UpdateAccountName renames the caller's account. Like the read methods the
// UserID is taken from the verified claim, so the repository's ownership scoping
// pins the rename to an account the caller owns — renaming another user's
// account reads as "account not found".
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

// UpdateAccountBalanceParams is the wire request for UpdateAccountBalance.
type UpdateAccountBalanceParams struct {
	AccountID string `json:"account_id"`
	// Delta is the signed amount to apply: positive to credit, negative to debit.
	Delta int64 `json:"delta"`
}

// UpdateAccountBalance applies a raw signed delta to an account balance. It is
// admin-only: this is the ledger-bypassing write path, intended for operator
// corrections, so it does not flow through the wallet's idempotency/audit unit
// of work. The permission map already gates this to admins (account:write:all);
// the explicit claim check here is defence in depth so the policy cannot drift
// if the map changes. The overdraft floor is still enforced by the repository's
// guarded UPDATE.
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
