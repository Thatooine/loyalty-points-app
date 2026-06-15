package wallet

import "context"

type TransactionRepository interface {
	Create(ctx context.Context, request CreateTransactionRequest) (*CreateTransactionResponse, error)

	List(ctx context.Context, request ListTransactionsRequest) (*ListTransactionsResponse, error)

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

// ListTransactionsRequest is the request for List. Listing is keyset-paginated:
// rows come back newest-first and the caller walks pages by echoing the
// previous response's NextCursor.
type ListTransactionsRequest struct {
	UserID string

	// PageSize caps how many transactions to return. Zero requests the server
	// default; values above the server maximum are clamped down.
	PageSize int

	// Cursor is an opaque token from a previous response's NextCursor. Empty
	// requests the first page.
	Cursor string
}

// ListTransactionsResponse is the response for List.
type ListTransactionsResponse struct {
	Transactions []Transaction

	// NextCursor is the token to pass as the next request's Cursor to fetch the
	// following page. Empty when the returned page is the last one.
	NextCursor string
}

// GetTransactionByIDRequest is the request for GetByID.
type GetTransactionByIDRequest struct {
	UserID string

	Ref string
}

// GetTransactionByIDResponse is the response for GetByID.
type GetTransactionByIDResponse struct {
	Transaction Transaction
}
