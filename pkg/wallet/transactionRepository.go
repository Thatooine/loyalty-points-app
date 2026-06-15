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

// ListTransactionsRequest is the request for List.
type ListTransactionsRequest struct {
	UserID string
}

// ListTransactionsResponse is the response for List.
type ListTransactionsResponse struct {
	Transactions []Transaction
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
