package ingest

import "time"

// Request is a JSON-RPC 2.0 request. gorilla/rpc/v2/json2 expects params to be
// an array whose single element is the method's argument object, so Params is
// always a one-element slice.
type Request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

// transactionParams is one transaction in the batch, built from a CSV row. The
// field names match the wallet adaptor's wire shape. occurred_at is omitted
// when blank so the server stamps it at processing time.
type transactionParams struct {
	Ref        string `json:"ref"`
	AccountID  string `json:"account_id"`
	Kind       string `json:"kind"`
	Points     int64  `json:"points"`
	OccurredAt string `json:"occurred_at,omitempty"`
}

// batchParams is the argument object for the wallet's batch method: the ordered
// list of transactions.
type batchParams struct {
	Transactions []transactionParams `json:"transactions"`
}

// BuildRequest turns the (already sorted) rows into a single JSON-RPC request
// calling method once with the whole ordered batch. The server applies the
// transactions in this slice order.
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
