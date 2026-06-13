// Package ingest turns a CSV batch file into a JSON-RPC 2.0 batch request,
// sends it to the loyalty-points server, and summarises the per-element
// results. It is a thin client: all business rules (idempotency, overdraft,
// audit) live on the server and are reached through the same RPC method the
// API exposes.
package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ExpectedHeader is the CSV header the batch file must have, per
// implementation-plan.html §8.
const ExpectedHeader = "ref,account_id,kind,points,occurred_at"

// Row is a well-formed CSV data row.
type Row struct {
	// Line is the 1-based data-row number (the header is not counted).
	Line       int
	Ref        string
	AccountID  string
	Kind       string
	Points     int64
	OccurredAt string
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

		rows = append(rows, Row{
			Line:       line,
			Ref:        ref,
			AccountID:  record[1],
			Kind:       record[2],
			Points:     points,
			OccurredAt: record[4],
		})
	}

	return rows, rowErrors, nil
}
