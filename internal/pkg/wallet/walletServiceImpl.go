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
	txManager    pkgSQL.TxManager
	accounts     pkgAccounts.AccountRepository
	transactions pkgWallet.TransactionRepository
	auditEntries pkgAudit.AuditEntryRepository
}

func NewWalletServiceImpl(
	txManager pkgSQL.TxManager,
	accounts pkgAccounts.AccountRepository,
	transactions pkgWallet.TransactionRepository,
	auditEntries pkgAudit.AuditEntryRepository,
) *WalletServiceImpl {
	return &WalletServiceImpl{
		txManager:    txManager,
		accounts:     accounts,
		transactions: transactions,
		auditEntries: auditEntries,
	}
}

func (s *WalletServiceImpl) ProcessTransaction(ctx context.Context, request pkgWallet.ProcessTransactionRequest) (*pkgWallet.ProcessTransactionResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for ProcessTransaction: %w", err)
	}

	delta := signedDelta(request.Kind, request.Points)
	now := time.Now().UTC()

	// Ownership gate: a non-admin actor may only transact on an account they
	// own. We resolve the account up front (an unscoped lookup so we can tell
	// "unknown account" apart from "not the owner" for the audit trail). Admins
	// bypass the check and may act on any account.
	if !request.ActorIsAdmin {
		account, err := s.accounts.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: request.AccountID})
		if err != nil {
			return s.rejectTransaction(ctx, request, delta, now, err)
		}
		if account.Account.UserID != request.Actor {
			return s.rejectTransaction(ctx, request, delta, now, errs.ErrForbidden)
		}
	}

	var resp *pkgWallet.ProcessTransactionResponse

	// One unit of work: ledger insert, balance update, and the accepted/
	// duplicate audit row all commit together or not at all.
	err := s.txManager.RunInTx(ctx, func(ctx context.Context) error {
		// 1. Idempotency: attempt the ledger insert FIRST. The unique
		//    constraint on ref is the dedupe mechanism — we never
		//    check-then-insert.
		created, err := s.transactions.Create(ctx, pkgWallet.CreateTransactionRequest{
			Transaction: pkgWallet.Transaction{
				Ref:        request.Ref,
				AccountID:  request.AccountID,
				Kind:       request.Kind,
				Points:     delta,
				OccurredAt: request.OccurredAt,
				RecordedAt: now,
				CreatedBy:  request.Actor,
			},
		})
		if errors.Is(err, errs.ErrDuplicateRef) {
			// Seen before: return the original outcome with Duplicate=true.
			// This is normal operation (client retry / file reprocessing),
			// not an error.
			original, err := s.transactions.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: request.Ref})
			if err != nil {
				return err
			}
			account, err := s.accounts.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: request.AccountID})
			if err != nil {
				return err
			}
			if _, err := s.auditEntries.Create(ctx, buildAuditEntry(request, delta, pkgAudit.OutcomeDuplicate, "duplicate", now)); err != nil {
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
		//    insert above so the ref stays free for a later retry.
		updated, err := s.accounts.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{
			AccountID: request.AccountID,
			Delta:     delta,
		})
		if err != nil {
			return err
		}

		// 3. Audit the accepted attempt in the same unit of work.
		if _, err := s.auditEntries.Create(ctx, buildAuditEntry(request, delta, pkgAudit.OutcomeAccepted, "ok", now)); err != nil {
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
		return s.rejectTransaction(ctx, request, delta, now, err)
	}

	return resp, nil
}

// rejectTransaction records a rejected audit entry on the plain context (so the
// trail survives the rolled-back unit of work, or a pre-flight rejection that
// never opened one) and returns the originating error.
func (s *WalletServiceImpl) rejectTransaction(ctx context.Context, request pkgWallet.ProcessTransactionRequest, delta int64, now time.Time, err error) (*pkgWallet.ProcessTransactionResponse, error) {
	if _, auditErr := s.auditEntries.Create(ctx, buildAuditEntry(request, delta, pkgAudit.OutcomeRejected, reasonFor(err), now)); auditErr != nil {
		return nil, auditErr
	}
	return nil, err
}

// signedDelta converts a request's points into the signed delta as applied:
// earn credits, spend debits, and adjust is already signed by the caller.
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
func buildAuditEntry(request pkgWallet.ProcessTransactionRequest, delta int64, outcome pkgAudit.Outcome, reason string, now time.Time) pkgAudit.CreateAuditEntryRequest {
	ref := request.Ref
	accountID := request.AccountID
	kind := string(request.Kind)
	points := delta

	return pkgAudit.CreateAuditEntryRequest{
		AuditEntry: pkgAudit.AuditEntry{
			Ref:       &ref,
			AccountID: &accountID,
			Kind:      &kind,
			Points:    &points,
			Source:    request.Source,
			Outcome:   outcome,
			Reason:    reason,
			Actor:     request.Actor,
			CreatedAt: now,
		},
	}
}
