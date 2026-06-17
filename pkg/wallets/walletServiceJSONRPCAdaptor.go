package wallets

import (
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

type WalletServiceJSONRPCAdaptor struct {
	walletService WalletService
}

func NewWalletServiceJSONRPCAdaptor(walletService WalletService) *WalletServiceJSONRPCAdaptor {
	return &WalletServiceJSONRPCAdaptor{walletService: walletService}
}

func (a *WalletServiceJSONRPCAdaptor) Name() string {
	return "Wallet"
}

type ProcessTransactionJSONRPCRequest struct {
	Ref        string    `json:"ref"`
	AccountID  string    `json:"account_id"`
	Kind       string    `json:"kind"`
	Points     int64     `json:"points"`
	OccurredAt time.Time `json:"occurred_at,omitempty"`
}

type ProcessTransactionJSONRPCResponse struct {
	Ref        string    `json:"ref"`
	AccountID  string    `json:"account_id"`
	Kind       string    `json:"kind"`
	Points     int64     `json:"points"`
	OccurredAt time.Time `json:"occurred_at"`
	RecordedAt time.Time `json:"recorded_at"`
	Balance    int64     `json:"balance"`
	Duplicate  bool      `json:"duplicate"`
}

func (a *WalletServiceJSONRPCAdaptor) ProcessTransaction(r *http.Request, params *ProcessTransactionJSONRPCRequest, result *ProcessTransactionJSONRPCResponse) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.walletService.ProcessTransaction(ctx, ProcessTransactionRequest{
		Ref:        params.Ref,
		AccountID:  params.AccountID,
		Kind:       Kind(params.Kind),
		Points:     params.Points,
		OccurredAt: params.OccurredAt,
		UserID:     claim.UserID,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("accountID", params.AccountID).Msg("wallet: process transaction failed")
		return mapProcessError(err)
	}

	fillProcessResult(result, resp)
	return nil
}

type EarnPointsJSONRPCRequest struct {
	Ref        string    `json:"ref"`
	AccountID  string    `json:"account_id"`
	Points     int64     `json:"points"`
	OccurredAt time.Time `json:"occurred_at,omitempty"`
}

// UserID is taken from the verified claim, never the wire.
func (a *WalletServiceJSONRPCAdaptor) EarnPoints(r *http.Request, params *EarnPointsJSONRPCRequest, result *ProcessTransactionJSONRPCResponse) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.walletService.EarnPoints(ctx, EarnPointsRequest{
		Ref:        params.Ref,
		AccountID:  params.AccountID,
		Points:     params.Points,
		OccurredAt: params.OccurredAt,
		UserID:     claim.UserID,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("accountID", params.AccountID).Msg("wallet: earn points failed")
		return mapProcessError(err)
	}

	fillProcessResult(result, resp)
	return nil
}

type SpendPointsJSONRPCRequest struct {
	Ref        string    `json:"ref"`
	AccountID  string    `json:"account_id"`
	Points     int64     `json:"points"`
	OccurredAt time.Time `json:"occurred_at,omitempty"`
}

func (a *WalletServiceJSONRPCAdaptor) SpendPoints(r *http.Request, params *SpendPointsJSONRPCRequest, result *ProcessTransactionJSONRPCResponse) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.walletService.SpendPoints(ctx, SpendPointsRequest{
		Ref:        params.Ref,
		AccountID:  params.AccountID,
		Points:     params.Points,
		OccurredAt: params.OccurredAt,
		UserID:     claim.UserID,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("accountID", params.AccountID).Msg("wallet: spend points failed")
		return mapProcessError(err)
	}

	fillProcessResult(result, resp)
	return nil
}

func mapProcessError(err error) error {
	switch {
	case errors.Is(err, errs.ErrForbidden):
		return errs.WithMessage(errs.ErrForbidden, "you do not own this account")
	case errors.Is(err, errs.ErrInsufficientBalance):
		return errs.WithMessage(errs.ErrInsufficientBalance, "insufficient balance")
	case errors.Is(err, errs.ErrNotFound):
		return errs.WithMessage(errs.ErrNotFound, "account not found")
	case errors.Is(err, errs.ErrInvalidArgument):
		return err
	default:
		return errs.WithMessage(errs.ErrInternal, "could not process transaction")
	}
}

func fillProcessResult(result *ProcessTransactionJSONRPCResponse, resp *ProcessTransactionResponse) {
	result.Ref = resp.Transaction.Ref
	result.AccountID = resp.Transaction.AccountID
	result.Kind = string(resp.Transaction.Kind)
	result.Points = resp.Transaction.Points
	result.OccurredAt = resp.Transaction.OccurredAt
	result.RecordedAt = resp.Transaction.RecordedAt
	result.Balance = resp.Balance
	result.Duplicate = resp.Duplicate
}

// Transactions are applied in array order; the CLI sorts by OccurredAt before sending.
type ProcessTransactionBatchParams struct {
	Transactions []ProcessTransactionJSONRPCRequest `json:"transactions"`
}

type BatchTransactionResult struct {
	Ref     string `json:"ref"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Balance int64  `json:"balance,omitempty"`
}

type BatchSummary struct {
	Accepted  int `json:"accepted"`
	Duplicate int `json:"duplicate"`
	Rejected  int `json:"rejected"`
}

type ProcessTransactionBatchResult struct {
	Results []BatchTransactionResult `json:"results"`
	Summary BatchSummary             `json:"summary"`
}

// Explicit admin check is defence in depth: the policy already gates this, but
// it must not drift if the permission map changes.
func (a *WalletServiceJSONRPCAdaptor) ProcessTransactionBatch(r *http.Request, params *ProcessTransactionBatchParams, result *ProcessTransactionBatchResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}
	if claim.Role != users.RoleAdmin {
		log.Ctx(ctx).Warn().Str("userID", claim.UserID).Msg("wallet: non-admin attempted batch ingestion")
		return errs.WithMessage(errs.ErrForbidden, "batch ingestion is admin-only")
	}

	batch := ProcessTransactionBatchRequest{
		Transactions: make([]ProcessTransactionRequest, 0, len(params.Transactions)),
	}
	for _, p := range params.Transactions {
		batch.Transactions = append(batch.Transactions, ProcessTransactionRequest{
			Ref:        p.Ref,
			AccountID:  p.AccountID,
			Kind:       Kind(p.Kind),
			Points:     p.Points,
			OccurredAt: p.OccurredAt,
			UserID:     claim.UserID,
		})
	}

	resp, err := a.walletService.ProcessTransactionBatch(ctx, batch)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("wallet: process transaction batch failed")
		return errs.WithMessage(errs.ErrInternal, "could not process transaction batch")
	}

	result.Results = make([]BatchTransactionResult, 0, len(resp.Results))
	for _, e := range resp.Results {
		result.Results = append(result.Results, BatchTransactionResult{
			Ref:     e.Ref,
			Status:  string(e.Outcome),
			Reason:  e.Reason,
			Balance: e.Balance,
		})
	}
	result.Summary = BatchSummary{
		Accepted:  resp.Accepted,
		Duplicate: resp.Duplicate,
		Rejected:  resp.Rejected,
	}
	return nil
}
