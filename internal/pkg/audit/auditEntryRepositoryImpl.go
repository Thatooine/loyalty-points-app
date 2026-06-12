package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audit"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/sqlite"
)

// AuditEntryRepositoryImpl is the SQLite implementation of
// audit.AuditEntryRepository. Every method resolves its executor from the
// context, so it runs inside an ambient transaction when one is present and
// against the pool otherwise.
type AuditEntryRepositoryImpl struct {
	db *sql.DB
}

func NewAuditEntryRepositoryImpl(db *sql.DB) *AuditEntryRepositoryImpl {
	return &AuditEntryRepositoryImpl{db: db}
}

func (r *AuditEntryRepositoryImpl) Create(ctx context.Context, request pkgAudit.CreateAuditEntryRequest) (*pkgAudit.CreateAuditEntryResponse, error) {
	exec := sqlite.ExecutorFromContext(ctx, r.db)

	entry := request.AuditEntry
	result, err := exec.ExecContext(ctx,
		`INSERT INTO audit_log (ref, account_id, kind, points, source, outcome, reason, actor, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Ref,
		entry.AccountID,
		entry.Kind,
		entry.Points,
		entry.Source,
		string(entry.Outcome),
		entry.Reason,
		entry.Actor,
		sqlite.FormatTime(entry.CreatedAt),
	)
	if err != nil {
		return nil, fmt.Errorf("could not insert audit entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("could not read audit entry id: %w", err)
	}
	entry.ID = id

	return &pkgAudit.CreateAuditEntryResponse{AuditEntry: entry}, nil
}

func (r *AuditEntryRepositoryImpl) List(ctx context.Context, request pkgAudit.ListAuditEntriesRequest) (*pkgAudit.ListAuditEntriesResponse, error) {
	exec := sqlite.ExecutorFromContext(ctx, r.db)

	rows, err := exec.QueryContext(ctx,
		`SELECT id, ref, account_id, kind, points, source, outcome, reason, actor, created_at
		 FROM audit_log
		 ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query audit entries: %w", err)
	}
	defer rows.Close()

	var entries []pkgAudit.AuditEntry
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

	return &pkgAudit.ListAuditEntriesResponse{AuditEntries: entries}, nil
}

func (r *AuditEntryRepositoryImpl) GetByID(ctx context.Context, request pkgAudit.GetAuditEntryByIDRequest) (*pkgAudit.GetAuditEntryByIDResponse, error) {
	exec := sqlite.ExecutorFromContext(ctx, r.db)

	row := exec.QueryRowContext(ctx,
		`SELECT id, ref, account_id, kind, points, source, outcome, reason, actor, created_at
		 FROM audit_log
		 WHERE id = ?`,
		request.ID,
	)

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
		&entry.Ref,
		&entry.AccountID,
		&entry.Kind,
		&entry.Points,
		&entry.Source,
		&outcome,
		&entry.Reason,
		&entry.Actor,
		&createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("could not scan audit entry: %w", err)
	}

	entry.Outcome = pkgAudit.Outcome(outcome)

	parsedCreatedAt, err := sqlite.ParseTime(createdAt)
	if err != nil {
		return nil, err
	}
	entry.CreatedAt = parsedCreatedAt

	return &entry, nil
}
