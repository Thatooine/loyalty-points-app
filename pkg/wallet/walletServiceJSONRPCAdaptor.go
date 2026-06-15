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
	Ref       string `json:"ref"`
	AccountID string `json:"account_id"`
	Kind      string `json:"kind"`
	Points    int64  `json:"points"`
	// OccurredAt is optional; when omitted the server stamps it at processing time.
	OccurredAt time.Time `json:"occurred_at,omitempty"`
}

// ProcessTransactionJSONRPCResponse is the wire response.
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
		return errors.New("unauthorized")
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

// EarnPointsJSONRPCRequest is the wire request for the earn convenience method.
// The Kind is implied by the method, so it is absent here.
type EarnPointsJSONRPCRequest struct {
	Ref       string `json:"ref"`
	AccountID string `json:"account_id"`
	Points    int64  `json:"points"`
	// OccurredAt is optional; when omitted the server stamps it at processing time.
	OccurredAt time.Time `json:"occurred_at,omitempty"`
}

// EarnPoints credits points to the caller's account. Like ProcessTransaction
// the Actor is taken from the verified claim, never from the client; the Kind
// is fixed to earn by the method.
func (a *WalletServiceJSONRPCAdaptor) EarnPoints(r *http.Request, params *EarnPointsJSONRPCRequest, result *ProcessTransactionJSONRPCResponse) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errors.New("unauthorized")
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

// SpendPointsJSONRPCRequest is the wire request for the spend convenience
// method. As with earn, the Kind is implied by the method.
type SpendPointsJSONRPCRequest struct {
	Ref       string `json:"ref"`
	AccountID string `json:"account_id"`
	Points    int64  `json:"points"`
	// OccurredAt is optional; when omitted the server stamps it at processing time.
	OccurredAt time.Time `json:"occurred_at,omitempty"`
}

// SpendPoints debits points from the caller's account. The Actor is taken from
// the verified claim and the Kind is fixed to spend by the method; the debit is
// subject to the balance floor enforced downstream.
func (a *WalletServiceJSONRPCAdaptor) SpendPoints(r *http.Request, params *SpendPointsJSONRPCRequest, result *ProcessTransactionJSONRPCResponse) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errors.New("unauthorized")
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

// mapProcessError translates a service-layer transaction error into the opaque,
// client-facing error returned over the wire. Shared by every method that runs
// through ProcessTransaction so the mapping stays in one place.
func mapProcessError(err error) error {
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

// fillProcessResult copies a service response onto the wire result. Shared by
// ProcessTransaction, EarnPoints, and SpendPoints, which all return the same
// shape.
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

// ProcessTransactionBatchParams is the wire request for the batch ingestion
// path. Transactions are applied in array order — the caller (the CLI) sorts
// them by OccurredAt, then line, before sending. As with the single method,
// Actor is taken from the verified claim, never from the client.
type ProcessTransactionBatchParams struct {
	Transactions []ProcessTransactionJSONRPCRequest `json:"transactions"`
}

// BatchTransactionResult is the wire outcome of one element of a batch.
type BatchTransactionResult struct {
	Ref     string `json:"ref"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Balance int64  `json:"balance,omitempty"`
}

// BatchSummary tallies a batch run.
type BatchSummary struct {
	Accepted  int `json:"accepted"`
	Duplicate int `json:"duplicate"`
	Rejected  int `json:"rejected"`
}

// ProcessTransactionBatchResult is the wire response: per-element results in
// input order plus summary tallies.
type ProcessTransactionBatchResult struct {
	Results []BatchTransactionResult `json:"results"`
	Summary BatchSummary             `json:"summary"`
}

// ProcessTransactionBatch applies an ordered batch in a single request. It is
// admin-only: batch ingestion is an operator action, so a member token is
// rejected. The permission map already gates this to admins (members lack the
// method, admins hold the wildcard); the explicit claim check here is defence
// in depth so the policy cannot drift if the map changes.
func (a *WalletServiceJSONRPCAdaptor) ProcessTransactionBatch(r *http.Request, params *ProcessTransactionBatchParams, result *ProcessTransactionBatchResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("wallet: no login claim in context for protected method")
		return errors.New("unauthorized")
	}
	if claim.Role != users.RoleAdmin {
		log.Ctx(ctx).Warn().Str("userID", claim.UserID).Msg("wallet: non-admin attempted batch ingestion")
		return errors.New("forbidden: batch ingestion is admin-only")
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
		return errors.New("could not process transaction batch")
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
