package sql

import (
	"context"
	"database/sql"
)

// Executor is the subset of database/sql operations shared by *sql.DB and
// *sql.Tx. Repositories run their SQL against an Executor so the same code
// works standalone (pool) or inside a unit of work (transaction).
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type TxContextKey struct{}

// ExecutorFromContext returns the transaction stored in ctx by RunInTx if
// present, otherwise the given database pool.
func ExecutorFromContext(ctx context.Context, db *sql.DB) Executor {
	if tx, ok := ctx.Value(TxContextKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}
