package ingest

import (
	"encoding/json"
	"strings"
	"testing"
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
}

func TestParseCSV_BadHeaderIsFatal(t *testing.T) {
	const data = "ref,account,kind,points,when\ntx-1,acc-1,earn,100,2026-06-01T10:00:00Z"
	if _, _, err := ParseCSV(strings.NewReader(data)); err == nil {
		t.Fatalf("ParseCSV() with bad header: error = nil, want error")
	}
}

func TestBuildBatch(t *testing.T) {
	rows := []Row{{Line: 1, Ref: "tx-1", AccountID: "acc-1", Kind: "earn", Points: 100, OccurredAt: "2026-06-01T10:00:00Z"}}

	batch := BuildBatch(rows, "Wallet.ProcessTransaction")
	if len(batch) != 1 {
		t.Fatalf("got %d requests, want 1", len(batch))
	}
	req := batch[0]
	if req.Version != "2.0" || req.Method != "Wallet.ProcessTransaction" || req.ID != 1 {
		t.Fatalf("unexpected request envelope: %+v", req)
	}
	if len(req.Params) != 1 {
		t.Fatalf("params must be a one-element array, got %d", len(req.Params))
	}

	// the single params element marshals with the expected field names
	encoded, _ := json.Marshal(req.Params[0])
	for _, want := range []string{`"ref":"tx-1"`, `"account_id":"acc-1"`, `"kind":"earn"`, `"points":100`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("params %s missing %s", encoded, want)
		}
	}
}

func TestSummarize(t *testing.T) {
	rows := []Row{
		{Line: 1, Ref: "tx-1"},
		{Line: 2, Ref: "tx-2"},
		{Line: 3, Ref: "tx-3"},
	}
	localErrors := []RowError{{Line: 4, Ref: "tx-4", Reason: "invalid points: \"abc\""}}
	responses := []Response{
		{ID: 1, Result: json.RawMessage(`{"duplicate":false}`)},
		{ID: 2, Result: json.RawMessage(`{"duplicate":true}`)},
		{ID: 3, Error: &RPCError{Code: -32002, Message: "insufficient balance"}},
	}

	s := Summarize("batch.csv", rows, localErrors, responses)

	if s.Sent != 3 {
		t.Fatalf("Sent = %d, want 3", s.Sent)
	}
	if s.Accepted != 1 || s.Duplicates != 1 || s.Rejected != 1 {
		t.Fatalf("tallies: accepted=%d duplicates=%d rejected=%d, want 1/1/1", s.Accepted, s.Duplicates, s.Rejected)
	}
	if s.LocalErrors != 1 {
		t.Fatalf("LocalErrors = %d, want 1", s.LocalErrors)
	}
	// one server rejection + one local error = two rejection lines
	if len(s.Rejections) != 2 {
		t.Fatalf("Rejections = %d, want 2", len(s.Rejections))
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
