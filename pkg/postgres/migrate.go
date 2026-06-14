package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies the embedded Postgres migrations in lexical filename order,
// skipping any already recorded in the schema_migrations table. It is safe to
// run at every startup.
//
// The pgx driver defaults to the extended query
// protocol, which rejects multiple statements in a single Exec. The migration
// files are authored as semicolon-terminated DDL statements, so each file is
// split and its statements executed one at a time inside the migration's
// transaction.
func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("could not create schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("could not read embedded migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		version := entry.Name()

		var applied int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, version,
		).Scan(&applied)
		if err != nil {
			return fmt.Errorf("could not check migration %s: %w", version, err)
		}
		if applied > 0 {
			continue
		}

		contents, err := migrationsFS.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("could not read migration %s: %w", version, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("could not begin transaction for migration %s: %w", version, err)
		}
		for _, stmt := range splitStatements(string(contents)) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("could not apply migration %s: %w", version, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
			version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("could not record migration %s: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("could not commit migration %s: %w", version, err)
		}
	}

	return nil
}

// splitStatements breaks a migration file into individual SQL statements on the
// semicolon terminator. Line comments are stripped first so a semicolon inside a
// "-- ..." comment is not mistaken for a statement terminator. The embedded
// migrations are authored without semicolons inside string literals or function
// bodies, so this is sufficient and avoids pulling in a full SQL parser.
func splitStatements(sqlText string) []string {
	var stripped strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		stripped.WriteString(line)
		stripped.WriteByte('\n')
	}

	parts := strings.Split(stripped.String(), ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			statements = append(statements, trimmed)
		}
	}
	return statements
}
