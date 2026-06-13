package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	sql2 "github.com/Thatooine/loyalty-points-app/pkg/sql"
)

// SQLiteTxManager implements TransactionManager over a SQLite database pool.
type SQLiteTxManager struct {
	db *sql.DB
}

func NewSQLiteTxManager(db *sql.DB) *SQLiteTxManager {
	return &SQLiteTxManager{db: db}
}

// RunInTx begins a transaction, stores it in a derived context, and runs fn.
// The transaction commits if fn returns nil and rolls back otherwise. If ctx
// already carries a transaction, fn joins it directly and commit/rollback is
// left to the outermost RunInTx call.
func (m *SQLiteTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(sql2.TxContextKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}

	if err := fn(context.WithValue(ctx, sql2.TxContextKey{}, tx)); err != nil {
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
