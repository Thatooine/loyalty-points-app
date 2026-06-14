package wallet

import (
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

// WalletServiceJSONRPCAdaptor exposes WalletService over JSON-RPC. It is a
// protected service: the authorization middleware authenticates the caller and
// places the verified login claim in the request context before any method is
// reached. The acting principal (Actor) and admin status are taken from that
// claim — never from client input — so a caller cannot transact as someone
// else.
type WalletServiceJSONRPCAdaptor struct {
	walletService WalletService
}

func NewWalletServiceJSONRPCAdaptor(walletService WalletService) *WalletServiceJSONRPCAdaptor {
	return &WalletServiceJSONRPCAdaptor{walletService: walletService}
}

func (a *WalletServiceJSONRPCAdaptor) Name() string {
	return "Wallet"
}

// ProcessTransactionParams is the wire request. Field names match the CSV batch
// shape the CLI sends (account_id, occurred_at) so a batch of these can be
// posted directly. Actor/Source are intentionally absent: they come from the
// verified claim, not the client.
type ProcessTransactionParams struct {
	Ref        string    `json:"ref"`
	AccountID  string    `json:"account_id"`
	Kind       string    `json:"kind"`
	Points     int64     `json:"points"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ProcessTransactionResult is the wire response.
type ProcessTransactionResult struct {
	Ref        string    `json:"ref"`
	AccountID  string    `json:"account_id"`
	Kind       string    `json:"kind"`
	Points     int64     `json:"points"`
	OccurredAt time.Time `json:"occurred_at"`
	RecordedAt time.Time `json:"recorded_at"`
	Balance    int64     `json:"balance"`
	Duplicate  bool      `json:"duplicate"`
}

func (a *WalletServiceJSONRPCAdaptor) ProcessTransaction(r *http.Request, params *ProcessTransactionParams, result *ProcessTransactionResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errors.New("unauthorized")
	}

	resp, err := a.walletService.ProcessTransaction(ctx, ProcessTransactionRequest{
		Ref:          params.Ref,
		AccountID:    params.AccountID,
		Kind:         Kind(params.Kind),
		Points:       params.Points,
		OccurredAt:   params.OccurredAt,
		Actor:        claim.UserID,
		ActorIsAdmin: claim.Role == users.RoleAdmin,
		Source:       "api",
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("accountID", params.AccountID).Msg("wallet: process transaction failed")
		switch {
		case errors.Is(err, errs.ErrForbidden):
			return errors.New("forbidden: you do not own this account")
		case errors.Is(err, errs.ErrInsufficientBalance):
			return errors.New("insufficient balance")
		case errors.Is(err, errs.ErrNotFound):
			return errors.New("account not found")
		default:
			return errors.New("could not process transaction")
		}
	}

	result.Ref = resp.Transaction.Ref
	result.AccountID = resp.Transaction.AccountID
	result.Kind = string(resp.Transaction.Kind)
	result.Points = resp.Transaction.Points
	result.OccurredAt = resp.Transaction.OccurredAt
	result.RecordedAt = resp.Transaction.RecordedAt
	result.Balance = resp.Balance
	result.Duplicate = resp.Duplicate
	return nil
}
