package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/sqlite"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
)

// TransactionRepositoryImpl is the SQLite implementation of
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
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	transaction := request.Transaction
	if transaction.ID == "" {
		transaction.ID = uuid.NewString()
	}
	_, err := exec.ExecContext(ctx,
		`INSERT INTO transactions (id, ref, account_id, kind, points, occurred_at, recorded_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		transaction.ID,
		transaction.Ref,
		transaction.AccountID,
		string(transaction.Kind),
		transaction.Points,
		time.FormatTime(transaction.OccurredAt),
		time.FormatTime(transaction.RecordedAt),
		transaction.CreatedBy,
	)
	if err != nil {
		if sqlite.IsUniqueConstraintViolation(err) {
			return nil, fmt.Errorf("transaction %s: %w", transaction.Ref, errs.ErrDuplicateRef)
		}
		if sqlite.IsForeignKeyConstraintViolation(err) {
			return nil, fmt.Errorf("account %s: %w", transaction.AccountID, errs.ErrNotFound)
		}
		return nil, fmt.Errorf("could not insert transaction: %w", err)
	}

	return &pkgWallet.CreateTransactionResponse{Transaction: transaction}, nil
}

func (r *TransactionRepositoryImpl) List(ctx context.Context, request pkgWallet.ListTransactionsRequest) (*pkgWallet.ListTransactionsResponse, error) {
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	rows, err := exec.QueryContext(ctx,
		`SELECT id, ref, account_id, kind, points, occurred_at, recorded_at, created_by
		 FROM transactions
		 ORDER BY recorded_at DESC, ref`,
	)
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
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	row := exec.QueryRowContext(ctx,
		`SELECT id, ref, account_id, kind, points, occurred_at, recorded_at, created_by
		 FROM transactions
		 WHERE ref = ?`,
		request.Ref,
	)

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
