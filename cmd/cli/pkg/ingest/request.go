package ingest

import "time"

// Request is a JSON-RPC 2.0 request. gorilla/rpc/v2/json2 expects params to be a
// one-element array whose element is the method's argument object.
type Request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type transactionParams struct {
	Ref        string `json:"ref"`
	AccountID  string `json:"account_id"`
	Kind       string `json:"kind"`
	Points     int64  `json:"points"`
	OccurredAt string `json:"occurred_at,omitempty"`
}

type batchParams struct {
	Transactions []transactionParams `json:"transactions"`
}

// BuildRequest builds one JSON-RPC request from the already-sorted rows; the
// server applies the transactions in this slice order.
func BuildRequest(rows []Row, method string) Request {
	transactions := make([]transactionParams, 0, len(rows))
	for _, row := range rows {
		occurredAt := ""
		if !row.OccurredAt.IsZero() {
			occurredAt = row.OccurredAt.Format(time.RFC3339)
		}
		transactions = append(transactions, transactionParams{
			Ref:        row.Ref,
			AccountID:  row.AccountID,
			Kind:       row.Kind,
			Points:     row.Points,
			OccurredAt: occurredAt,
		})
	}

	return Request{
		Version: "2.0",
		Method:  method,
		Params:  []any{batchParams{Transactions: transactions}},
		ID:      1,
	}
}
