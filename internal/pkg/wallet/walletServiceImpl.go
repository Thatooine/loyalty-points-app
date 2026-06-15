package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audit"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
)

// WalletServiceImpl is the heart of the system (plan §5.1): every write path
// flows through ProcessTransaction, which composes the account, transaction,
// and audit repositories inside one unit of work so the invariants —
// idempotency, the overdraft floor, and the audit trail — hold once and are
// tested once. The repositories beneath it stay policy-free.
type WalletServiceImpl struct {
	txManager             pkgSQL.TxManager
	accountRepository     pkgAccounts.AccountRepository
	transactionRepository pkgWallet.TransactionRepository
	auditEntryRepository  pkgAudit.AuditEntryRepository
}

func NewWalletServiceImpl(
	txManager pkgSQL.TxManager,
	accountRepository pkgAccounts.AccountRepository,
	transactionRepository pkgWallet.TransactionRepository,
	auditEntryRepository pkgAudit.AuditEntryRepository,
) *WalletServiceImpl {
	return &WalletServiceImpl{
		txManager:             txManager,
		accountRepository:     accountRepository,
		transactionRepository: transactionRepository,
		auditEntryRepository:  auditEntryRepository,
	}
}

func (s *WalletServiceImpl) ProcessTransaction(ctx context.Context, request pkgWallet.ProcessTransactionRequest) (*pkgWallet.ProcessTransactionResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for ProcessTransaction: %w", err)
	}

	delta := signedDelta(request.Kind, request.Points)
	now := time.Now().UTC()

	// OccurredAt is optional on the request: when the caller does not supply it,
	// stamp it with the processing time so it is never persisted as the zero
	// value.
	occurredAt := request.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}

	// Resolve the account, scoped to the caller. Ownership is enforced by the
	// scoped read itself: an account the caller does not own reads as
	// ErrNotFound, indistinguishable from a missing one (no existence leak), so
	// there is no separate ownership gate. The resolved owner id is stamped on
	// the ledger and audit rows. A failed lookup rejects the attempt.
	account, err := s.accountRepository.GetByID(
		ctx,
		pkgAccounts.GetAccountByIDRequest{
			AccountID: request.AccountID,
			UserID:    request.UserID,
		},
	)
	if err != nil {
		return s.rejectTransaction(ctx, request, delta, now, nil, err)
	}
	ownerID := account.Account.OwnerID

	var resp *pkgWallet.ProcessTransactionResponse

	// One unit of work: ledger insert, balance update, and the accepted/
	// duplicate audit row all commit together or not at all.
	err = s.txManager.RunInTx(ctx, func(ctx context.Context) error {
		// 1. Idempotency: attempt the ledger insert FIRST. The unique
		//    constraint on ref is the dedupe mechanism — we never
		//    check-then-insert.
		created, err := s.transactionRepository.Create(ctx, pkgWallet.CreateTransactionRequest{
			Transaction: pkgWallet.Transaction{
				Ref:        request.Ref,
				AccountID:  request.AccountID,
				OwnerID:    ownerID,
				Kind:       request.Kind,
				Points:     delta,
				OccurredAt: occurredAt,
				RecordedAt: now,
				CreatedBy:  request.UserID,
			},
		})
		if errors.Is(err, errs.ErrDuplicateRef) {
			// Seen before: return the original outcome with Duplicate=true.
			// This is normal operation (client retry / file reprocessing),
			// not an error.
			original, err := s.transactionRepository.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: request.Ref, UserID: request.UserID})
			if err != nil {
				return err
			}
			account, err := s.accountRepository.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: request.AccountID, UserID: request.UserID})
			if err != nil {
				return err
			}
			if _, err := s.auditEntryRepository.Create(ctx, buildAuditEntry(request, delta, &ownerID, pkgAudit.OutcomeDuplicate, "duplicate", now)); err != nil {
				return err
			}
			resp = &pkgWallet.ProcessTransactionResponse{
				Transaction: original.Transaction,
				Balance:     account.Account.Balance,
				Duplicate:   true,
			}
			return nil
		}
		if err != nil {
			return err
		}

		// 2. Balance floor: the repository's single overdraft-guarded UPDATE
		//    returns ErrInsufficientBalance / ErrNotFound on zero rows. On
		//    error the whole unit of work rolls back, discarding the ledger
		//    insert above so the ref stays free for a later retry. The update is
		//    scoped to the owner so the repository re-enforces ownership in SQL.
		updated, err := s.accountRepository.UpdateAccountBalance(
			ctx,
			pkgAccounts.UpdateAccountBalanceRequest{
				AccountID: request.AccountID,
				Delta:     delta,
				UserID:    ownerID,
			})
		if err != nil {
			return err
		}

		// 3. Audit the accepted attempt in the same unit of work.
		if _, err := s.auditEntryRepository.Create(ctx,
			buildAuditEntry(
				request,
				delta,
				&ownerID,
				pkgAudit.OutcomeAccepted,
				"ok",
				now,
			)); err != nil {
			return err
		}

		resp = &pkgWallet.ProcessTransactionResponse{
			Transaction: created.Transaction,
			Balance:     updated.Balance,
			Duplicate:   false,
		}
		return nil
	})

	// 4. Rejected attempts: the unit of work rolled back, so write the audit
	//    row on the plain context (pool executor) — outside the rolled-back
	//    transaction — so the trail survives the rejection.
	if err != nil {
		return s.rejectTransaction(ctx, request, delta, now, &ownerID, err)
	}

	return resp, nil
}

// EarnPoints credits points to an account by constructing an earn transaction
// and delegating to ProcessTransaction, so it inherits the single write path's
// idempotency, balance floor, ownership scoping, and audit trail. The only
// thing it adds is fixing the Kind to KindEarn from the method itself.
func (s *WalletServiceImpl) EarnPoints(ctx context.Context, request pkgWallet.EarnPointsRequest) (*pkgWallet.ProcessTransactionResponse, error) {
	return s.ProcessTransaction(ctx, pkgWallet.ProcessTransactionRequest{
		UserID:     request.UserID,
		Ref:        request.Ref,
		AccountID:  request.AccountID,
		Kind:       pkgWallet.KindEarn,
		Points:     request.Points,
		OccurredAt: request.OccurredAt,
	})
}

// SpendPoints debits points from an account by constructing a spend transaction
// and delegating to ProcessTransaction. As with EarnPoints the only addition is
// fixing the Kind — here KindSpend — so the debit runs through the same guarded
// write path and is subject to the balance floor.
func (s *WalletServiceImpl) SpendPoints(ctx context.Context, request pkgWallet.SpendPointsRequest) (*pkgWallet.ProcessTransactionResponse, error) {
	return s.ProcessTransaction(ctx, pkgWallet.ProcessTransactionRequest{
		UserID:     request.UserID,
		Ref:        request.Ref,
		AccountID:  request.AccountID,
		Kind:       pkgWallet.KindSpend,
		Points:     request.Points,
		OccurredAt: request.OccurredAt,
	})
}

// ProcessTransactionBatch applies an ordered batch sequentially. Each element
// reuses the single-transaction path — inheriting idempotency, the overdraft
// floor, ownership checks, and the audit trail — so there is no second code
// path to keep in sync. Elements are processed strictly in slice order and a
// per-element rejection is captured in the result rather than aborting the
// batch, so one bad row never sinks the rest.
func (s *WalletServiceImpl) ProcessTransactionBatch(ctx context.Context, request pkgWallet.ProcessTransactionBatchRequest) (*pkgWallet.ProcessTransactionBatchResponse, error) {
	resp := &pkgWallet.ProcessTransactionBatchResponse{
		Results: make([]pkgWallet.BatchElementResult, 0, len(request.Transactions)),
	}

	for _, txRequest := range request.Transactions {
		result, err := s.ProcessTransaction(ctx, txRequest)
		switch {
		case err != nil:
			resp.Results = append(resp.Results, pkgWallet.BatchElementResult{
				Ref:     txRequest.Ref,
				Outcome: pkgWallet.BatchOutcomeRejected,
				Reason:  reasonFor(err),
			})
			resp.Rejected++
		case result.Duplicate:
			resp.Results = append(resp.Results, pkgWallet.BatchElementResult{
				Ref:     txRequest.Ref,
				Outcome: pkgWallet.BatchOutcomeDuplicate,
				Balance: result.Balance,
			})
			resp.Duplicate++
		default:
			resp.Results = append(resp.Results, pkgWallet.BatchElementResult{
				Ref:     txRequest.Ref,
				Outcome: pkgWallet.BatchOutcomeAccepted,
				Balance: result.Balance,
			})
			resp.Accepted++
		}
	}

	return resp, nil
}

// rejectTransaction records a rejected audit entry on the plain context (so the
// trail survives the rolled-back unit of work, or a pre-flight rejection that
// never opened one) and returns the originating error.
func (s *WalletServiceImpl) rejectTransaction(ctx context.Context, request pkgWallet.ProcessTransactionRequest, delta int64, now time.Time, ownerID *string, err error) (*pkgWallet.ProcessTransactionResponse, error) {
	if _, auditErr := s.auditEntryRepository.Create(
		ctx,
		buildAuditEntry(
			request,
			delta,
			ownerID,
			pkgAudit.OutcomeRejected,
			reasonFor(err),
			now),
	); auditErr != nil {
		return nil, auditErr
	}
	return nil, err
}

// signedDelta converts a request's points into the signed delta as applied:
// earn credits, spend debits.
func signedDelta(kind pkgWallet.Kind, points int64) int64 {
	if kind == pkgWallet.KindSpend {
		return -points
	}
	return points
}

// reasonFor maps a processing error to a human-readable audit reason.
func reasonFor(err error) string {
	switch {
	case errors.Is(err, errs.ErrInsufficientBalance):
		return "insufficient balance"
	case errors.Is(err, errs.ErrNotFound):
		return "unknown account"
	case errors.Is(err, errs.ErrForbidden):
		return "not account owner"
	default:
		return err.Error()
	}
}

// buildAuditEntry assembles an audit row echoing the attempted payload. Points
// records the signed delta as applied, matching the ledger.
func buildAuditEntry(request pkgWallet.ProcessTransactionRequest, delta int64, ownerID *string, outcome pkgAudit.Outcome, reason string, now time.Time) pkgAudit.CreateAuditEntryRequest {
	ref := request.Ref
	accountID := request.AccountID
	kind := string(request.Kind)
	points := delta

	return pkgAudit.CreateAuditEntryRequest{
		AuditEntry: pkgAudit.AuditEntry{
			TransactionRef: &ref,
			AccountID:      &accountID,
			OwnerID:        ownerID,
			Kind:           &kind,
			Points:         &points,
			Outcome:        outcome,
			Reason:         reason,
			UserID:         request.UserID,
			CreatedAt:      now,
		},
	}
}
