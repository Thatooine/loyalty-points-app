package time

import (
	"fmt"
	"time"
)

// Timestamps are stored as RFC3339Nano UTC TEXT, chosen because it is
// human-readable and lexicographically sortable.

func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse timestamp %q: %w", s, err)
	}
	return t, nil
}
