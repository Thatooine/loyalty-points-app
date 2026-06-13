package ingest

// Request is one element of a JSON-RPC 2.0 batch. gorilla/rpc/v2/json2 expects
// params to be an array whose single element is the method's argument object,
// so Params is always a one-element slice.
type Request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

// transactionParams is the argument object for the wallet's process-transaction
// method, built from one CSV row. The JSON field names are provisional until
// the wallet RPC adaptor lands (Task 2/3); align them with the adaptor's
// request struct then.
type transactionParams struct {
	Ref        string `json:"ref"`
	AccountID  string `json:"account_id"`
	Kind       string `json:"kind"`
	Points     int64  `json:"points"`
	OccurredAt string `json:"occurred_at"`
}

// BuildBatch turns parsed rows into a JSON-RPC batch calling method once per
// row. Each element gets a 1-based id so responses can be correlated back.
func BuildBatch(rows []Row, method string) []Request {
	batch := make([]Request, 0, len(rows))
	for i, row := range rows {
		batch = append(batch, Request{
			Version: "2.0",
			Method:  method,
			Params: []any{transactionParams{
				Ref:        row.Ref,
				AccountID:  row.AccountID,
				Kind:       row.Kind,
				Points:     row.Points,
				OccurredAt: row.OccurredAt,
			}},
			ID: i + 1,
		})
	}
	return batch
}
