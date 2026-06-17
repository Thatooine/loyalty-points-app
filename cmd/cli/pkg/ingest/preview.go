package ingest

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// Preview renders, for a dry run, the ordered transactions plus a client-side
// breakdown. It reports no accepted/duplicate/rejected outcome: those depend on
// server state and are only knowable from a real run.
func Preview(rows []Row) string {
	var b strings.Builder

	b.WriteString("\napply order (server processes top to bottom):\n")
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  #\tref\taccount_id\tkind\tpoints\toccurred_at")
	for i, row := range rows {
		when := "(server-stamped)"
		if !row.OccurredAt.IsZero() {
			when = row.OccurredAt.Format(time.RFC3339)
		}
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\t%s\n",
			i+1, row.Ref, row.AccountID, row.Kind, signedPoints(row), when)
	}
	_ = tw.Flush()

	accounts := make(map[string]struct{})
	refCounts := make(map[string]int)
	var earnCount, spendCount, stamped int
	var earnPts, spendPts int64
	for _, row := range rows {
		accounts[row.AccountID] = struct{}{}
		refCounts[row.Ref]++
		if row.OccurredAt.IsZero() {
			stamped++
		}
		if row.Kind == "spend" {
			spendCount++
			spendPts += row.Points
		} else {
			earnCount++
			earnPts += row.Points
		}
	}

	var dupRefs []string
	for ref, n := range refCounts {
		if n > 1 {
			dupRefs = append(dupRefs, ref)
		}
	}
	sort.Strings(dupRefs)

	fmt.Fprintf(&b, "\nbatch breakdown:\n")
	fmt.Fprintf(&b, "  accounts:       %d\n", len(accounts))
	fmt.Fprintf(&b, "  earns:          %d  (+%d pts)\n", earnCount, earnPts)
	fmt.Fprintf(&b, "  spends:         %d  (-%d pts)\n", spendCount, spendPts)
	if stamped > 0 {
		fmt.Fprintf(&b, "  server-stamped: %d  (blank occurred_at)\n", stamped)
	}
	if len(dupRefs) > 0 {
		// The server keeps the first and dedupes the rest via UNIQUE(ref); surfaced
		// here so an accidental in-file repeat is visible before sending.
		fmt.Fprintf(&b, "  duplicate refs: %d  (%s)\n", len(dupRefs), strings.Join(dupRefs, ", "))
	}

	return b.String()
}

func signedPoints(row Row) string {
	if row.Kind == "spend" {
		return fmt.Sprintf("-%d", row.Points)
	}
	return fmt.Sprintf("+%d", row.Points)
}
