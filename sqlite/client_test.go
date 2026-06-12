package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewClient(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"

	db, err := NewClient(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer db.Close()

	var result int
	if err := db.QueryRow("SELECT 1").Scan(&result); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}
}
