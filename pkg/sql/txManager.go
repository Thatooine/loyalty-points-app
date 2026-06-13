package sql

import "context"

// TxManager is the unit-of-work port: fn runs with a transaction
// stored in its context, so every repository call inside fn participates in
// the same atomic transaction.
type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
