package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// stdlib registers the "pgx" driver with database/sql, so the existing
	// repository code (which is written against database/sql) works unchanged.
	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewClient(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	return db, nil
}
