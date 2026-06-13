package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Summary is the CLI's tally of a batch run, built from the server's
// per-element responses (or, on a dry run, from the parsed rows alone).
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
// JSON-RPC error.
type Rejection struct {
	Line   int
	Ref    string
	Reason string
}

// Summarize correlates responses back to rows by id and tallies the outcomes.
// Pass responses == nil for a dry run: rows are counted as "to send" and only
// local errors are reported.
func Summarize(file string, rows []Row, localErrors []RowError, responses []Response) Summary {
	summary := Summary{File: file, Sent: len(rows), LocalErrors: len(localErrors)}

	for _, e := range localErrors {
		summary.Rejections = append(summary.Rejections, Rejection{Line: e.Line, Ref: e.Ref, Reason: e.Reason})
	}

	byID := make(map[int]Row, len(rows))
	for i, row := range rows {
		byID[i+1] = row
	}

	for _, resp := range responses {
		row := byID[resp.ID]
		if resp.Error != nil {
			summary.Rejected++
			summary.Rejections = append(summary.Rejections, Rejection{Line: row.Line, Ref: row.Ref, Reason: resp.Error.Message})
			continue
		}

		var result struct {
			Duplicate bool `json:"duplicate"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Duplicate {
			summary.Duplicates++
		} else {
			summary.Accepted++
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
