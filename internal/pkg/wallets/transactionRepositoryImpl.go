package wallets

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallets"
)

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

	query := `SELECT id, ref, account_id, owner_id, kind, points, occurred_at, recorded_at, created_by
		 FROM transactions`
	var args []any
	var conds []string

	// Ownership scoping mirrors the account repository: holding
	// transaction:read:all lists every owner's transactions; otherwise the WHERE
	// clause restricts the listing to request.UserID.
	if !authorization.IsGranted(ctx, authorization.PermTransactionReadAll) {
		args = append(args, request.UserID)
		conds = append(conds, fmt.Sprintf("owner_id = $%d", len(args)))
	}

	// Keyset seek: continue strictly after the row the cursor names, under the
	// ORDER BY (recorded_at DESC, ref ASC). The predicate spells out that mixed
	// ordering rather than using a row-value comparison, which would assume both
	// columns ascending. recorded_at is RFC3339Nano TEXT, so comparing against
	// the cursor's stored representation is the same lexical order the ORDER BY
	// uses.
	if request.Cursor != "" {
		cursorRecordedAt, cursorRef, err := decodeTransactionCursor(request.Cursor)
		if err != nil {
			return nil, err
		}
		tsIdx, refIdx := len(args)+1, len(args)+2
		args = append(args, cursorRecordedAt, cursorRef)
		conds = append(conds, fmt.Sprintf("(recorded_at < $%d OR (recorded_at = $%d AND ref > $%d))", tsIdx, tsIdx, refIdx))
	}

	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}

	// Fetch one more than the page size: getting the extra row is how we learn
	// another page exists, without a second COUNT query.
	limit := clampPageSize(request.PageSize)
	args = append(args, limit+1)
	query += fmt.Sprintf(" ORDER BY recorded_at DESC, ref LIMIT $%d", len(args))

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

	// The sentinel extra row means there is a further page. Trim it off and hand
	// back a cursor built from the last row actually returned.
	resp := &pkgWallet.ListTransactionsResponse{}
	if len(transactions) > limit {
		transactions = transactions[:limit]
		last := transactions[len(transactions)-1]
		resp.NextCursor = encodeTransactionCursor(time.FormatTime(last.RecordedAt), last.Ref)
	}
	resp.Transactions = transactions

	return resp, nil
}

// Page-size policy for List: zero means the default, and any larger request is
// clamped down so a single call can never pull an unbounded result set.
const (
	defaultPageSize = 50
	maxPageSize     = 200
)

func clampPageSize(n int) int {
	switch {
	case n <= 0:
		return defaultPageSize
	case n > maxPageSize:
		return maxPageSize
	default:
		return n
	}
}

// transactionCursorSeparator joins the two cursor components. A NUL byte cannot
// appear in an RFC3339Nano timestamp or a ref, so it is an unambiguous delimiter.
const transactionCursorSeparator = "\x00"

// encodeTransactionCursor packs the (recorded_at, ref) keyset position into one
// opaque, URL-safe token. recordedAt is the stored RFC3339Nano string so it
// compares byte-for-byte against the column.
func encodeTransactionCursor(recordedAt, ref string) string {
	return base64.URLEncoding.EncodeToString([]byte(recordedAt + transactionCursorSeparator + ref))
}

// decodeTransactionCursor reverses encodeTransactionCursor. A token that is not
// valid base64, is missing a component, or carries an unparseable timestamp is a
// caller error (ErrInvalidArgument), not a server fault.
func decodeTransactionCursor(cursor string) (recordedAt, ref string, err error) {
	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", fmt.Errorf("decode cursor: %w", errs.ErrInvalidArgument)
	}
	parts := strings.SplitN(string(raw), transactionCursorSeparator, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("malformed cursor: %w", errs.ErrInvalidArgument)
	}
	if _, err := time.ParseTime(parts[0]); err != nil {
		return "", "", fmt.Errorf("malformed cursor timestamp: %w", errs.ErrInvalidArgument)
	}
	return parts[0], parts[1], nil
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
