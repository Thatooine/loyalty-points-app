package ingest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseCSV(t *testing.T) {
	const data = `ref,account_id,kind,points,occurred_at
tx-1,acc-1,earn,100,2026-06-01T10:00:00Z
tx-2,acc-1,spend,50,2026-06-01T11:00:00Z
tx-3,acc-2,earn,abc,2026-06-01T12:00:00Z`

	rows, rowErrors, err := ParseCSV(strings.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d valid rows, want 2", len(rows))
	}
	if len(rowErrors) != 1 {
		t.Fatalf("got %d row errors, want 1", len(rowErrors))
	}
	if rowErrors[0].Line != 3 || rowErrors[0].Ref != "tx-3" {
		t.Fatalf("unexpected row error: %+v", rowErrors[0])
	}
	if rows[0].Ref != "tx-1" || rows[0].Points != 100 {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if !rows[0].OccurredAt.Equal(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected first row OccurredAt: %v", rows[0].OccurredAt)
	}
}

func TestParseCSV_BadHeaderIsFatal(t *testing.T) {
	const data = "ref,account,kind,points,when\ntx-1,acc-1,earn,100,2026-06-01T10:00:00Z"
	if _, _, err := ParseCSV(strings.NewReader(data)); err == nil {
		t.Fatalf("ParseCSV() with bad header: error = nil, want error")
	}
}

func TestParseCSV_BlankOccurredAtIsAllowed(t *testing.T) {
	const data = "ref,account_id,kind,points,occurred_at\ntx-1,acc-1,earn,100,"

	rows, rowErrors, err := ParseCSV(strings.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}
	if len(rowErrors) != 0 {
		t.Fatalf("got %d row errors, want 0", len(rowErrors))
	}
	if len(rows) != 1 || !rows[0].OccurredAt.IsZero() {
		t.Fatalf("blank occurred_at should parse to zero time, got %+v", rows)
	}
}

func TestParseCSV_InvalidOccurredAtIsLocalError(t *testing.T) {
	const data = "ref,account_id,kind,points,occurred_at\ntx-1,acc-1,earn,100,not-a-time"

	rows, rowErrors, err := ParseCSV(strings.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("got %d valid rows, want 0", len(rows))
	}
	if len(rowErrors) != 1 || rowErrors[0].Ref != "tx-1" {
		t.Fatalf("expected one local error for tx-1, got %+v", rowErrors)
	}
}

func TestSortRows(t *testing.T) {
	rows := []Row{
		{Line: 1, Ref: "spend", OccurredAt: time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)},
		{Line: 2, Ref: "earn", OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)},
		{Line: 3, Ref: "blank"}, // zero time sorts first
		{Line: 4, Ref: "tie", OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)},
	}

	SortRows(rows)

	gotOrder := []string{rows[0].Ref, rows[1].Ref, rows[2].Ref, rows[3].Ref}
	wantOrder := []string{"blank", "earn", "tie", "spend"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("sorted order = %v, want %v", gotOrder, wantOrder)
		}
	}
}

func TestBuildRequest(t *testing.T) {
	rows := []Row{
		{Line: 1, Ref: "tx-1", AccountID: "acc-1", Kind: "earn", Points: 100, OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)},
		{Line: 2, Ref: "tx-2", AccountID: "acc-1", Kind: "spend", Points: 50}, // blank occurred_at
	}

	request := BuildRequest(rows, "Wallet.ProcessTransactionBatch")
	if request.Version != "2.0" || request.Method != "Wallet.ProcessTransactionBatch" || request.ID != 1 {
		t.Fatalf("unexpected request envelope: %+v", request)
	}
	if len(request.Params) != 1 {
		t.Fatalf("params must be a one-element array, got %d", len(request.Params))
	}

	encoded, _ := json.Marshal(request.Params[0])
	// the batch wraps the transactions under "transactions"
	for _, want := range []string{`"transactions":[`, `"ref":"tx-1"`, `"account_id":"acc-1"`, `"kind":"earn"`, `"points":100`, `"occurred_at":"2026-06-01T10:00:00Z"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("params %s missing %s", encoded, want)
		}
	}
	// the blank occurred_at row omits the field entirely
	if strings.Count(string(encoded), `"occurred_at"`) != 1 {
		t.Fatalf("blank occurred_at should be omitted, got %s", encoded)
	}
}

func TestSummarize(t *testing.T) {
	rows := []Row{
		{Line: 1, Ref: "tx-1"},
		{Line: 2, Ref: "tx-2"},
		{Line: 3, Ref: "tx-3"},
	}
	localErrors := []RowError{{Line: 4, Ref: "tx-4", Reason: "invalid points: \"abc\""}}
	response := &Response{
		ID: 1,
		Result: json.RawMessage(`{
			"results": [
				{"ref": "tx-1", "status": "accepted"},
				{"ref": "tx-2", "status": "duplicate"},
				{"ref": "tx-3", "status": "rejected", "reason": "insufficient balance"}
			],
			"summary": {"accepted": 1, "duplicate": 1, "rejected": 1}
		}`),
	}

	s := Summarize("batch.csv", rows, localErrors, response)

	if s.Sent != 3 {
		t.Fatalf("Sent = %d, want 3", s.Sent)
	}
	if s.Accepted != 1 || s.Duplicates != 1 || s.Rejected != 1 {
		t.Fatalf("tallies: accepted=%d duplicates=%d rejected=%d, want 1/1/1", s.Accepted, s.Duplicates, s.Rejected)
	}
	if s.LocalErrors != 1 {
		t.Fatalf("LocalErrors = %d, want 1", s.LocalErrors)
	}
	// one server rejection (correlated to line 3) + one local error = two lines
	if len(s.Rejections) != 2 {
		t.Fatalf("Rejections = %d, want 2", len(s.Rejections))
	}
	var foundServerRejection bool
	for _, r := range s.Rejections {
		if r.Ref == "tx-3" && r.Line == 3 && r.Reason == "insufficient balance" {
			foundServerRejection = true
		}
	}
	if !foundServerRejection {
		t.Fatalf("server rejection not correlated to its row: %+v", s.Rejections)
	}
}

func TestSummarize_WholeBatchError(t *testing.T) {
	rows := []Row{{Line: 1, Ref: "tx-1"}, {Line: 2, Ref: "tx-2"}}
	response := &Response{Error: &RPCError{Code: -32000, Message: "forbidden: batch ingestion is admin-only"}}

	s := Summarize("batch.csv", rows, nil, response)

	if s.Rejected != 2 || len(s.Rejections) != 2 {
		t.Fatalf("whole-batch error should reject all rows: rejected=%d rejections=%d", s.Rejected, len(s.Rejections))
	}
}

func TestSummary_FormatDryRun(t *testing.T) {
	rows := []Row{{Line: 1, Ref: "tx-1"}}
	out := Summarize("batch.csv", rows, nil, nil).Format(true)

	if !strings.Contains(out, "DRY RUN") {
		t.Fatalf("dry-run output missing DRY RUN banner:\n%s", out)
	}
	if !strings.Contains(out, "rows to send: 1") {
		t.Fatalf("dry-run output missing row count:\n%s", out)
	}
	if strings.Contains(out, "accepted:") {
		t.Fatalf("dry-run output should not report accepted:\n%s", out)
	}
}
