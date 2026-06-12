package sqlite

import (
	"fmt"
	"time"
)

// Timestamps are stored as RFC3339 UTC TEXT — SQLite-idiomatic,
// human-readable, and lexicographically sortable.

// FormatTime encodes a time for storage in a TEXT column.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// ParseTime decodes a TEXT column value written by FormatTime.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse timestamp %q: %w", s, err)
	}
	return t, nil
}
