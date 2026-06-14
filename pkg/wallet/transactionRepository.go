package wallet

import "context"

// TransactionRepository is the persistence port for the append-only points
// ledger — no Update or Delete by design. Methods participate in an ambient
// transaction when one is present in the context (see
// sql.TxManager), and run against the pool otherwise.
type TransactionRepository interface {
	// Create appends a transaction to the ledger. A transaction with the
	// same Ref results in errs.ErrDuplicateRef — the unique constraint is
	// the dedupe mechanism, arbitrating races at the database. A reference to
	// an account that does not exist results in errs.ErrNotFound.
	Create(ctx context.Context, request CreateTransactionRequest) (*CreateTransactionResponse, error)

	// List returns all transactions, newest first by RecordedAt.
	List(ctx context.Context, request ListTransactionsRequest) (*ListTransactionsResponse, error)

	// GetByID returns the transaction with the given Ref, or
	// errs.ErrNotFound.
	GetByID(ctx context.Context, request GetTransactionByIDRequest) (*GetTransactionByIDResponse, error)
}

// CreateTransactionRequest is the request for Create.
type CreateTransactionRequest struct {
	Transaction Transaction
}

// CreateTransactionResponse is the response for Create.
type CreateTransactionResponse struct {
	Transaction Transaction
}

// ListTransactionsRequest is the request for List.
type ListTransactionsRequest struct {
}

// ListTransactionsResponse is the response for List.
type ListTransactionsResponse struct {
	Transactions []Transaction
}

// GetTransactionByIDRequest is the request for GetByID.
type GetTransactionByIDRequest struct {
	Ref string
}

// GetTransactionByIDResponse is the response for GetByID.
type GetTransactionByIDResponse struct {
	Transaction Transaction
}
