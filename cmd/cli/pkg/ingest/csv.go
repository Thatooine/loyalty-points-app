// Package ingest turns a CSV batch file into a single JSON-RPC request to the
// wallet's batch ingestion method, sends it to the loyalty-points server, and
// summarises the per-element results. It is a thin client: all business rules
// (idempotency, overdraft, audit) live on the server and are reached through
// the same transaction core the single-transaction API uses.
//
// A dedicated batch method (rather than a JSON-RPC 2.0 batch array) is used on
// purpose: the JSON-RPC spec lets a server process a batch in any order and
// concurrently, and the gorilla json2 codec does not decode batch arrays at
// all. Since the overdraft floor makes each write order-dependent, ordering has
// to be guaranteed — so the whole batch travels as one ordered request.
package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ExpectedHeader is the CSV header the batch file must have, per
// implementation-plan.html §8.
const ExpectedHeader = "ref,account_id,kind,points,occurred_at"

// Row is a well-formed CSV data row.
type Row struct {
	// Line is the 1-based data-row number (the header is not counted). It is the
	// deterministic tiebreaker when two rows share an OccurredAt.
	Line      int
	Ref       string
	AccountID string
	Kind      string
	Points    int64
	// OccurredAt is the parsed business timestamp. It is the zero value when the
	// CSV cell was blank — the server then stamps it at processing time.
	OccurredAt time.Time
}

// RowError is a row that failed local parsing before any request was built. It
// is reported in the summary and never sent to the server.
type RowError struct {
	Line   int
	Ref    string
	Reason string
}

// ParseCSV reads the batch, validating the header. Well-formed rows and rows
// that failed local parsing are returned separately; a bad header is fatal.
// occurred_at is optional: a blank cell parses to the zero time (the server
// stamps it on processing), while a non-blank cell that is not RFC 3339 is a
// local error so a typo is caught before it is sent.
func ParseCSV(r io.Reader) ([]Row, []RowError, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("could not read CSV header: %w", err)
	}
	if got := strings.Join(header, ","); got != ExpectedHeader {
		return nil, nil, fmt.Errorf("invalid CSV header: got %q, want %q", got, ExpectedHeader)
	}

	var rows []Row
	var rowErrors []RowError
	line := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			rowErrors = append(rowErrors, RowError{Line: line, Reason: fmt.Sprintf("malformed row: %v", err)})
			continue
		}

		ref := record[0]
		points, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		if err != nil {
			rowErrors = append(rowErrors, RowError{Line: line, Ref: ref, Reason: fmt.Sprintf("invalid points: %q", record[3])})
			continue
		}

		var occurredAt time.Time
		if raw := strings.TrimSpace(record[4]); raw != "" {
			occurredAt, err = time.Parse(time.RFC3339, raw)
			if err != nil {
				rowErrors = append(rowErrors, RowError{Line: line, Ref: ref, Reason: fmt.Sprintf("invalid occurred_at: %q (want RFC 3339)", record[4])})
				continue
			}
		}

		rows = append(rows, Row{
			Line:       line,
			Ref:        ref,
			AccountID:  record[1],
			Kind:       record[2],
			Points:     points,
			OccurredAt: occurredAt,
		})
	}

	return rows, rowErrors, nil
}

// SortRows orders rows into the chronology the server will apply them in:
// ascending OccurredAt, with the original line number as a deterministic
// tiebreaker. Rows with a blank (zero) OccurredAt sort first. Sorting client
// side is what lets the order-dependent overdraft floor see, e.g., an earn
// before the spend it funds, regardless of the file's row order.
func SortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].OccurredAt.Equal(rows[j].OccurredAt) {
			return rows[i].Line < rows[j].Line
		}
		return rows[i].OccurredAt.Before(rows[j].OccurredAt)
	})
}
