package accounts

import (
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// The new account's owner is taken from the verified claim, never the wire, so
// a caller can only open an account for themselves.
type AccountOpenerJSONRPCAdaptor struct {
	opener AccountOpener
}

func NewAccountOpenerJSONRPCAdaptor(opener AccountOpener) *AccountOpenerJSONRPCAdaptor {
	return &AccountOpenerJSONRPCAdaptor{opener: opener}
}

func (a *AccountOpenerJSONRPCAdaptor) Name() string {
	return "AccountOpener"
}

type OpenAccountParams struct {
	Name string `json:"name"`
}

type OpenAccountResult struct {
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *AccountOpenerJSONRPCAdaptor) OpenAccount(r *http.Request, params *OpenAccountParams, result *OpenAccountResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("accountOpener: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.opener.OpenAccount(ctx, OpenAccountRequest{
		UserID: claim.UserID,
		Name:   params.Name,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("userID", claim.UserID).Msg("accountOpener: open account failed")
		if errors.Is(err, errs.ErrInvalidArgument) {
			return err
		}
		return errs.WithMessage(errs.ErrInternal, "could not open account")
	}

	result.AccountID = resp.Account.ID
	result.Name = resp.Account.Name
	result.OwnerID = resp.Account.OwnerID
	result.Balance = resp.Account.Balance
	result.CreatedAt = resp.Account.CreatedAt
	return nil
}
