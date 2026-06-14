package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audit"
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
		`INSERT INTO audit_log (ref, account_id, kind, points, source, outcome, reason, actor, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		entry.Ref,
		entry.AccountID,
		entry.Kind,
		entry.Points,
		entry.Source,
		string(entry.Outcome),
		entry.Reason,
		entry.Actor,
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

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

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
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetByID: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	row := exec.QueryRowContext(ctx,
		`SELECT id, ref, account_id, kind, points, source, outcome, reason, actor, created_at
		 FROM audit_log
		 WHERE id = $1`,
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

	parsedCreatedAt, err := time.ParseTime(createdAt)
	if err != nil {
		return nil, err
	}
	entry.CreatedAt = parsedCreatedAt

	return &entry, nil
}
