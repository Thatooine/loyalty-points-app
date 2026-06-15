package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
)

// TransactionRepositoryImpl is the Postgres implementation of
// wallet.TransactionRepository. Every method resolves its executor from the
// context, so it runs inside an ambient transaction when one is present and
// against the pool otherwise.
type TransactionRepositoryImpl struct {
	db *sql.DB
}

func NewTransactionRepositoryImpl(db *sql.DB) *TransactionRepositoryImpl {
	return &TransactionRepositoryImpl{db: db}
}

func (r *TransactionRepositoryImpl) Create(ctx context.Context, request pkgWallet.CreateTransactionRequest) (*pkgWallet.CreateTransactionResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for Create: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	transaction := request.Transaction
	if transaction.ID == "" {
		transaction.ID = uuid.NewString()
	}
	// ON CONFLICT (ref) DO NOTHING rather than letting the UNIQUE constraint
	// raise: in Postgres a raised error aborts the surrounding transaction
	// (SQLSTATE 25P02), but the wallet's idempotency flow catches ErrDuplicateRef
	// and then keeps reading the original row in the SAME transaction. Swallowing
	// the conflict keeps the transaction usable; a zero row count is the duplicate
	// signal.
	result, err := exec.ExecContext(ctx,
		`INSERT INTO transactions (id, ref, account_id, owner_id, kind, points, occurred_at, recorded_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (ref) DO NOTHING`,
		transaction.ID,
		transaction.Ref,
		transaction.AccountID,
		transaction.OwnerID,
		string(transaction.Kind),
		transaction.Points,
		time.FormatTime(transaction.OccurredAt),
		time.FormatTime(transaction.RecordedAt),
		transaction.CreatedBy,
	)
	if err != nil {
		if postgres.IsForeignKeyConstraintViolation(err) {
			return nil, fmt.Errorf("account %s: %w", transaction.AccountID, errs.ErrNotFound)
		}
		return nil, fmt.Errorf("could not insert transaction: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("could not read affected rows: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("transaction %s: %w", transaction.Ref, errs.ErrDuplicateRef)
	}

	return &pkgWallet.CreateTransactionResponse{Transaction: transaction}, nil
}

func (r *TransactionRepositoryImpl) List(ctx context.Context, request pkgWallet.ListTransactionsRequest) (*pkgWallet.ListTransactionsResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for List: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping mirrors the account repository: holding
	// transaction:read:all lists every owner's transactions; otherwise the WHERE
	// clause restricts the listing to request.UserID.
	query := `SELECT id, ref, account_id, owner_id, kind, points, occurred_at, recorded_at, created_by
		 FROM transactions`
	var args []any
	if !authorization.IsGranted(ctx, authorization.PermTransactionReadAll) {
		query += ` WHERE owner_id = $1`
		args = append(args, request.UserID)
	}
	query += ` ORDER BY recorded_at DESC, ref`

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []pkgWallet.Transaction
	for rows.Next() {
		transaction, err := scanTransaction(rows.Scan)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, *transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate transactions: %w", err)
	}

	return &pkgWallet.ListTransactionsResponse{Transactions: transactions}, nil
}

func (r *TransactionRepositoryImpl) GetByID(ctx context.Context, request pkgWallet.GetTransactionByIDRequest) (*pkgWallet.GetTransactionByIDResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetByID: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping mirrors the account repository: holding
	// transaction:read:all reads across owners; otherwise a non-owner gets the
	// same ErrNotFound as for a missing row.
	query := `SELECT id, ref, account_id, owner_id, kind, points, occurred_at, recorded_at, created_by
		 FROM transactions
		 WHERE ref = $1`
	args := []any{request.Ref}
	if !authorization.IsGranted(ctx, authorization.PermTransactionReadAll) {
		query += ` AND owner_id = $2`
		args = append(args, request.UserID)
	}

	row := exec.QueryRowContext(ctx, query, args...)

	transaction, err := scanTransaction(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("transaction %s: %w", request.Ref, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgWallet.GetTransactionByIDResponse{Transaction: *transaction}, nil
}
func scanTransaction(scan func(dest ...any) error) (*pkgWallet.Transaction, error) {
	var transaction pkgWallet.Transaction
	var kind, occurredAt, recordedAt string

	if err := scan(
		&transaction.ID,
		&transaction.Ref,
		&transaction.AccountID,
		&transaction.OwnerID,
		&kind,
		&transaction.Points,
		&occurredAt,
		&recordedAt,
		&transaction.CreatedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("could not scan transaction: %w", err)
	}

	transaction.Kind = pkgWallet.Kind(kind)

	parsedOccurredAt, err := time.ParseTime(occurredAt)
	if err != nil {
		return nil, err
	}
	transaction.OccurredAt = parsedOccurredAt

	parsedRecordedAt, err := time.ParseTime(recordedAt)
	if err != nil {
		return nil, err
	}
	transaction.RecordedAt = parsedRecordedAt

	return &transaction, nil
}
