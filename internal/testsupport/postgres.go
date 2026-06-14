// Package testsupport holds helpers shared across the project's tests. It is
// imported only from _test.go files; it is not part of the production build
// surface.
package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
)

// DSNEnv is the environment variable naming the Postgres instance the DB-backed
// tests run against (e.g. the docker-compose container). When it is unset those
// tests skip, so a plain `go test ./...` stays green without a database.
const DSNEnv = "TEST_POSTGRES_DSN"

var schemaCounter atomic.Int64

// NewPostgresDB returns a *sql.DB pointed at a fresh, isolated schema in the
// Postgres instance named by TEST_POSTGRES_DSN, skipping the test when that
// variable is unset. Each call provisions its own schema and migrates into it,
// so tests never observe one another's rows — even across packages that the Go
// test runner executes in parallel — and the schema is dropped on cleanup.
func NewPostgresDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv(DSNEnv)
	if dsn == "" {
		t.Skipf("%s not set; skipping DB-backed test (run `docker compose up -d` and export %s)", DSNEnv, DSNEnv)
	}

	// A name unique across processes (UnixNano) and within one (counter).
	schema := fmt.Sprintf("test_%d_%d", time.Now().UnixNano(), schemaCounter.Add(1))

	admin, err := postgres.NewClient(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to %s: %v", DSNEnv, err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema %s: %v", schema, err)
	}
	_ = admin.Close()

	// Every connection in this pool resolves unqualified names to the new
	// schema, so the migrations create the tables there.
	db, err := postgres.NewClient(ctx, withSearchPath(dsn, schema))
	if err != nil {
		t.Fatalf("connect with search_path: %v", err)
	}
	if err := postgres.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		cleanup, err := postgres.NewClient(ctx, dsn)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	})

	return db
}

// withSearchPath appends a search_path runtime parameter to the DSN so the pgx
// driver sets it on every connection it opens.
func withSearchPath(dsn, schema string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema
}
