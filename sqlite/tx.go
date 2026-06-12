package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// Executor is the subset of database/sql operations shared by *sql.DB and
// *sql.Tx. Repositories run their SQL against an Executor so the same code
// works standalone (pool) or inside a unit of work (transaction).
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txContextKey struct{}

// ExecutorFromContext returns the transaction stored in ctx by RunInTx if
// present, otherwise the given database pool.
func ExecutorFromContext(ctx context.Context, db *sql.DB) Executor {
	if tx, ok := ctx.Value(txContextKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// TransactionManager is the unit-of-work port: fn runs with a transaction
// stored in its context, so every repository call inside fn participates in
// the same atomic transaction.
type TransactionManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// TxManager implements TransactionManager over a SQLite database pool.
type TxManager struct {
	db *sql.DB
}

func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

// RunInTx begins a transaction, stores it in a derived context, and runs fn.
// The transaction commits if fn returns nil and rolls back otherwise. If ctx
// already carries a transaction, fn joins it directly and commit/rollback is
// left to the outermost RunInTx call.
func (m *TxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txContextKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}

	if err := fn(context.WithValue(ctx, txContextKey{}, tx)); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("could not roll back transaction: %w (original error: %w)", rollbackErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

	return nil
}
