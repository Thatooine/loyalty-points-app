package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
)

// AuditEntryRepositoryImpl is the Postgres implementation of
// audit.AuditEntryRepository. Create reads back the generated identity with an
// INSERT ... RETURNING id (pgx has no LastInsertId). Every method resolves its
// executor from the context, so it runs inside an ambient transaction when one
// is present and against the pool otherwise.
type AuditEntryRepositoryImpl struct {
	db *sql.DB
}

func NewAuditEntryRepositoryImpl(db *sql.DB) *AuditEntryRepositoryImpl {
	return &AuditEntryRepositoryImpl{db: db}
}

func (r *AuditEntryRepositoryImpl) Create(ctx context.Context, request pkgAudit.CreateAuditEntryRequest) (*pkgAudit.CreateAuditEntryResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for Create: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	entry := request.AuditEntry
	// pgx (extended protocol) has no LastInsertId; RETURNING id is the Postgres
	// idiom for reading back the generated identity.
	var id int64
	err := exec.QueryRowContext(ctx,
		`INSERT INTO audit_entries (transaction_ref, account_id, owner_id, kind, points, outcome, reason, user_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		entry.TransactionRef,
		entry.AccountID,
		entry.OwnerID,
		entry.Kind,
		entry.Points,
		string(entry.Outcome),
		entry.Reason,
		entry.UserID,
		time.FormatTime(entry.CreatedAt),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("could not insert audit entry: %w", err)
	}
	entry.ID = id

	return &pkgAudit.CreateAuditEntryResponse{AuditEntry: entry}, nil
}

func (r *AuditEntryRepositoryImpl) List(ctx context.Context, request pkgAudit.ListAuditEntriesRequest) (*pkgAudit.ListAuditEntriesResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for List: %w", err)
	}

	// Ownership scoping mirrors the account repository: holding audit:read:all
	// lists every owner's entries; otherwise the WHERE clause restricts the
	// listing to request.UserID.
	query := `SELECT id, transaction_ref, account_id, owner_id, kind, points, outcome, reason, user_id, created_at
		 FROM audit_entries`
	var args []any
	if !authorization.IsGranted(ctx, authorization.PermAuditReadAll) {
		query += ` WHERE owner_id = $1`
		args = append(args, request.UserID)
	}
	query += ` ORDER BY id`

	entries, err := r.queryEntries(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return &pkgAudit.ListAuditEntriesResponse{AuditEntries: entries}, nil
}

func (r *AuditEntryRepositoryImpl) ListByTransactionRef(ctx context.Context, request pkgAudit.ListAuditEntriesByTransactionRefRequest) (*pkgAudit.ListAuditEntriesByTransactionRefResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for ListByTransactionRef: %w", err)
	}

	query := `SELECT id, transaction_ref, account_id, owner_id, kind, points, outcome, reason, user_id, created_at
		 FROM audit_entries
		 WHERE transaction_ref = $1`
	args := []any{request.TransactionRef}
	if !authorization.IsGranted(ctx, authorization.PermAuditReadAll) {
		query += ` AND owner_id = $2`
		args = append(args, request.UserID)
	}
	query += ` ORDER BY id`

	entries, err := r.queryEntries(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return &pkgAudit.ListAuditEntriesByTransactionRefResponse{AuditEntries: entries}, nil
}

func (r *AuditEntryRepositoryImpl) ListByAccountID(ctx context.Context, request pkgAudit.ListAuditEntriesByAccountIDRequest) (*pkgAudit.ListAuditEntriesByAccountIDResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for ListByAccountID: %w", err)
	}

	query := `SELECT id, transaction_ref, account_id, owner_id, kind, points, outcome, reason, user_id, created_at
		 FROM audit_entries
		 WHERE account_id = $1`
	args := []any{request.AccountID}
	if !authorization.IsGranted(ctx, authorization.PermAuditReadAll) {
		query += ` AND owner_id = $2`
		args = append(args, request.UserID)
	}
	query += ` ORDER BY id`

	entries, err := r.queryEntries(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return &pkgAudit.ListAuditEntriesByAccountIDResponse{AuditEntries: entries}, nil
}

// queryEntries runs a SELECT returning the standard audit_entries column set and
// scans every row into AuditEntry values. It returns a non-nil empty slice when
// the query matches nothing.
func (r *AuditEntryRepositoryImpl) queryEntries(ctx context.Context, query string, args ...any) ([]pkgAudit.AuditEntry, error) {
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query audit entries: %w", err)
	}
	defer rows.Close()

	entries := []pkgAudit.AuditEntry{}
	for rows.Next() {
		entry, err := scanAuditEntry(rows.Scan)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate audit entries: %w", err)
	}

	return entries, nil
}

func (r *AuditEntryRepositoryImpl) GetByID(ctx context.Context, request pkgAudit.GetAuditEntryByIDRequest) (*pkgAudit.GetAuditEntryByIDResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetByID: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	query := `SELECT id, transaction_ref, account_id, owner_id, kind, points, outcome, reason, user_id, created_at
		 FROM audit_entries
		 WHERE id = $1`
	args := []any{request.ID}
	if !authorization.IsGranted(ctx, authorization.PermAuditReadAll) {
		query += ` AND owner_id = $2`
		args = append(args, request.UserID)
	}

	row := exec.QueryRowContext(ctx, query, args...)

	entry, err := scanAuditEntry(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("audit entry %d: %w", request.ID, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgAudit.GetAuditEntryByIDResponse{AuditEntry: *entry}, nil
}
func scanAuditEntry(scan func(dest ...any) error) (*pkgAudit.AuditEntry, error) {
	var entry pkgAudit.AuditEntry
	var outcome, createdAt string

	if err := scan(
		&entry.ID,
		&entry.TransactionRef,
		&entry.AccountID,
		&entry.OwnerID,
		&entry.Kind,
		&entry.Points,
		&outcome,
		&entry.Reason,
		&entry.UserID,
		&createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("could not scan audit entry: %w", err)
	}

	entry.Outcome = pkgAudit.Outcome(outcome)

	parsedCreatedAt, err := time.ParseTime(createdAt)
	if err != nil {
		return nil, err
	}
	entry.CreatedAt = parsedCreatedAt

	return &entry, nil
}
