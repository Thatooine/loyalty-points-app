package accounts

import (
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

// AccountOpenerJSONRPCAdaptor exposes AccountOpener over JSON-RPC. It is a
// protected service: the authorization middleware places the verified login
// claim in the request context. The new account's owner is taken from that
// claim — never from the wire — so an authenticated member can only open an
// account for themselves.
type AccountOpenerJSONRPCAdaptor struct {
	opener AccountOpener
}

func NewAccountOpenerJSONRPCAdaptor(opener AccountOpener) *AccountOpenerJSONRPCAdaptor {
	return &AccountOpenerJSONRPCAdaptor{opener: opener}
}

func (a *AccountOpenerJSONRPCAdaptor) Name() string {
	return "AccountOpener"
}

// OpenAccountParams is the wire request for OpenAccount. The owner is not on the
// wire: it is always the calling user, resolved from the login claim.
type OpenAccountParams struct {
	// Name is the optional display name for the new account; the service
	// defaults it when blank.
	Name string `json:"name"`
}

// OpenAccountResult is the wire response for OpenAccount.
type OpenAccountResult struct {
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

// OpenAccount opens a new wallet for the calling user. The caller is taken from
// the verified login claim, so OwnerID is pinned to the claim's UserID and no
// caller can open an account for anyone else.
func (a *AccountOpenerJSONRPCAdaptor) OpenAccount(r *http.Request, params *OpenAccountParams, result *OpenAccountResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("accountOpener: no login claim in context for protected method")
		return errors.New("unauthorized")
	}

	resp, err := a.opener.OpenAccount(ctx, OpenAccountRequest{
		UserID: claim.UserID,
		Name:   params.Name,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("userID", claim.UserID).Msg("accountOpener: open account failed")
		return errors.New("could not open account")
	}

	result.AccountID = resp.Account.ID
	result.Name = resp.Account.Name
	result.OwnerID = resp.Account.OwnerID
	result.Balance = resp.Account.Balance
	result.CreatedAt = resp.Account.CreatedAt
	return nil
}
