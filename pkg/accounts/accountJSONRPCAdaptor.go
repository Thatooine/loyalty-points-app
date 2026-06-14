package accounts

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// AccountJSONRPCAdaptor exposes read access to accounts over JSON-RPC. It is a
// protected service: the authorization middleware places the verified login
// claim in the request context. Every lookup is scoped to the caller's user id
// taken from that claim, so a caller can only read their own accounts —
// requesting another user's account id yields "account not found", with no way
// to tell it apart from a genuinely missing one.
type AccountJSONRPCAdaptor struct {
	accounts AccountRepository
}

func NewAccountJSONRPCAdaptor(accounts AccountRepository) *AccountJSONRPCAdaptor {
	return &AccountJSONRPCAdaptor{accounts: accounts}
}

func (a *AccountJSONRPCAdaptor) Name() string {
	return "Account"
}

// GetByIDParams is the wire request for GetByID and GetAccountBalance.
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

func (a *AccountJSONRPCAdaptor) GetByID(r *http.Request, params *GetByIDParams, result *AccountResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("account: no login claim in context for protected method")
		return errors.New("unauthorized")
	}

	resp, err := a.accounts.GetByID(ctx, GetAccountByIDRequest{
		AccountID: params.AccountID,
		UserID:    claim.UserID,
	})
	if err != nil {
		return notFoundOrInternal(ctx, err, params.AccountID)
	}

	result.ID = resp.Account.ID
	result.UserID = resp.Account.UserID
	result.Name = resp.Account.Name
	result.Balance = resp.Account.Balance
	result.CreatedAt = resp.Account.CreatedAt
	return nil
}

func (a *AccountJSONRPCAdaptor) GetAccountBalance(r *http.Request, params *GetByIDParams, result *BalanceResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("account: no login claim in context for protected method")
		return errors.New("unauthorized")
	}

	resp, err := a.accounts.GetAccountBalance(ctx, GetAccountBalanceRequest{
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

func notFoundOrInternal(ctx context.Context, err error, accountID string) error {
	log.Ctx(ctx).Warn().Err(err).Str("accountID", accountID).Msg("account: lookup failed")
	if errors.Is(err, errs.ErrNotFound) {
		return errors.New("account not found")
	}
	return errors.New("could not retrieve account")
}
