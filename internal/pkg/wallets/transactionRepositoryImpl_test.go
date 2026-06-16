package wallets

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// These are the pure-logic tests for the transaction repository: the cursor
// codec and page-size clamping. They need no database. The DB-backed behaviour
// (idempotency via UNIQUE(ref), ownership scoping, keyset pagination over real
// rows, transactional atomicity) is exercised by the endpoint integration suite
// in tests/, which runs against a live server and Postgres.

func TestTransactionCursorRoundTrip(t *testing.T) {
	recordedAt := "2026-06-01T10:00:01Z"
	ref := "tx-001"

	gotTS, gotRef, err := decodeTransactionCursor(encodeTransactionCursor(recordedAt, ref))
	if err != nil {
		t.Fatalf("decode(encode()) error = %v", err)
	}
	if gotTS != recordedAt || gotRef != ref {
		t.Fatalf("round trip = (%q, %q), want (%q, %q)", gotTS, gotRef, recordedAt, ref)
	}
}

func TestDecodeTransactionCursorRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"not base64":        "%%%",
		"missing separator": base64.URLEncoding.EncodeToString([]byte("2026-06-01T10:00:01Z")),
		"empty ref":         base64.URLEncoding.EncodeToString([]byte("2026-06-01T10:00:01Z\x00")),
		"bad timestamp":     base64.URLEncoding.EncodeToString([]byte("not-a-time\x00tx-001")),
	}
	for name, cursor := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decodeTransactionCursor(cursor); !errors.Is(err, errs.ErrInvalidArgument) {
				t.Fatalf("decode(%q) error = %v, want errs.ErrInvalidArgument", cursor, err)
			}
		})
	}
}

func TestClampPageSize(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, defaultPageSize},
		{-5, defaultPageSize},
		{10, 10},
		{maxPageSize, maxPageSize},
		{maxPageSize + 1, maxPageSize},
	}
	for _, c := range cases {
		if got := clampPageSize(c.in); got != c.want {
			t.Fatalf("clampPageSize(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
