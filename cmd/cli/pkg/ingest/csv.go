// Package ingest turns a CSV batch file into a single ordered JSON-RPC request
// to the wallet's batch ingestion method and summarises the per-element results.
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

const ExpectedHeader = "ref,account_id,kind,points,occurred_at"

type Row struct {
	// Line is the deterministic tiebreaker when two rows share an OccurredAt.
	Line      int
	Ref       string
	AccountID string
	Kind      string
	Points    int64
	// OccurredAt is the zero value when the CSV cell was blank; the server then
	// stamps it at processing time.
	OccurredAt time.Time
}

// RowError is a row that failed local parsing and is never sent to the server.
type RowError struct {
	Line   int
	Ref    string
	Reason string
}

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

// SortRows sorts by ascending OccurredAt (line number as tiebreaker) so the
// order-dependent overdraft floor sees, e.g., an earn before the spend it funds.
func SortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].OccurredAt.Equal(rows[j].OccurredAt) {
			return rows[i].Line < rows[j].Line
		}
		return rows[i].OccurredAt.Before(rows[j].OccurredAt)
	})
}
