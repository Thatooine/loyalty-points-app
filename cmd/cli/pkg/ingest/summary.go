package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Summary is the CLI's tally of a batch run, built from the server's batch
// response (or, on a dry run, from the parsed rows alone).
type Summary struct {
	File        string
	Sent        int // well-formed rows (sent to the server, or that would be on a dry run)
	Accepted    int
	Duplicates  int
	Rejected    int
	LocalErrors int // rows that failed local CSV parsing, never sent
	Rejections  []Rejection
}

// Rejection is a per-row failure — either a local parse error or a server-side
// rejection.
type Rejection struct {
	Line   int
	Ref    string
	Reason string
}

// batchResult mirrors the wallet adaptor's ProcessTransactionBatchResult wire
// shape.
type batchResult struct {
	Results []struct {
		Ref    string `json:"ref"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"results"`
	Summary struct {
		Accepted  int `json:"accepted"`
		Duplicate int `json:"duplicate"`
		Rejected  int `json:"rejected"`
	} `json:"summary"`
}

// Summarize tallies the outcome of a batch run. Pass response == nil for a dry
// run: rows are counted as "to send" and only local errors are reported.
func Summarize(file string, rows []Row, localErrors []RowError, response *Response) Summary {
	summary := Summary{File: file, Sent: len(rows), LocalErrors: len(localErrors)}

	for _, e := range localErrors {
		summary.Rejections = append(summary.Rejections, Rejection{Line: e.Line, Ref: e.Ref, Reason: e.Reason})
	}

	if response == nil {
		return summary
	}

	lineByRef := make(map[string]int, len(rows))
	for _, row := range rows {
		lineByRef[row.Ref] = row.Line
	}

	// A transport- or method-level error rejects the whole batch (e.g. a
	// non-admin token, or a malformed request). Every sent row is reported
	// rejected with that reason.
	if response.Error != nil {
		summary.Rejected = len(rows)
		for _, row := range rows {
			summary.Rejections = append(summary.Rejections, Rejection{Line: row.Line, Ref: row.Ref, Reason: response.Error.Message})
		}
		return summary
	}

	var result batchResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		summary.Rejected = len(rows)
		for _, row := range rows {
			summary.Rejections = append(summary.Rejections, Rejection{Line: row.Line, Ref: row.Ref, Reason: fmt.Sprintf("could not decode batch result: %v", err)})
		}
		return summary
	}

	summary.Accepted = result.Summary.Accepted
	summary.Duplicates = result.Summary.Duplicate
	summary.Rejected = result.Summary.Rejected

	for _, e := range result.Results {
		if e.Status == "rejected" {
			summary.Rejections = append(summary.Rejections, Rejection{Line: lineByRef[e.Ref], Ref: e.Ref, Reason: e.Reason})
		}
	}

	return summary
}

// Format renders the summary for the terminal.
func (s Summary) Format(dryRun bool) string {
	var b strings.Builder
	if dryRun {
		fmt.Fprintln(&b, "DRY RUN — no changes written")
	}
	fmt.Fprintf(&b, "file:         %s\n", s.File)

	if dryRun {
		fmt.Fprintf(&b, "rows to send: %d\n", s.Sent)
	} else {
		fmt.Fprintf(&b, "processed:    %d\n", s.Sent)
		fmt.Fprintf(&b, "accepted:     %d\n", s.Accepted)
		fmt.Fprintf(&b, "duplicates:   %d\n", s.Duplicates)
		fmt.Fprintf(&b, "rejected:     %d\n", s.Rejected)
	}
	if s.LocalErrors > 0 {
		fmt.Fprintf(&b, "local errors: %d\n", s.LocalErrors)
	}

	if len(s.Rejections) > 0 {
		fmt.Fprintln(&b, "\nrejections:")
		for _, r := range s.Rejections {
			ref := r.Ref
			if ref == "" {
				ref = "-"
			}
			fmt.Fprintf(&b, "  row %d (ref %s): %s\n", r.Line, ref, r.Reason)
		}
	}

	return b.String()
}
