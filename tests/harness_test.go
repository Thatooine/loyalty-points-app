// Package tests holds black-box integration tests that exercise the JSON-RPC
// API over HTTP against an already-running server — the Go equivalent of
// scripts/test_register_login.sh.
//
// The tests assume the server is already up. Point them at it with:
//
//	LOYALTY_API_URL   JSON-RPC endpoint   (default http://localhost:8080/api)
//	LOYALTY_DB_PATH   optional SQLite file path; when set, the persistence test
//	                  additionally reads the DB directly to confirm the bcrypt
//	                  hash and the opened wallet account.
//
// Run with:  go test ./tests/...    (after starting the server)
// If the server is unreachable the tests skip rather than fail, so a plain
// `go test ./...` without a running server stays green.
package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	// registers the modernc sqlite driver under the name "sqlite"
	"github.com/Thatooine/loyalty-points-app/pkg/sqlite"
)

const defaultAPIURL = "http://localhost:8080/api"

// apiClient talks to a running server over HTTP.
type apiClient struct {
	baseURL string
	db      *sql.DB // non-nil only when LOYALTY_DB_PATH is set
}

// setup returns a client for the running server, skipping the test if the
// server cannot be reached. When LOYALTY_DB_PATH is set it also opens the DB
// read-only for direct persistence assertions.
func setup(t *testing.T) *apiClient {
	t.Helper()

	baseURL := os.Getenv("LOYALTY_API_URL")
	if baseURL == "" {
		baseURL = defaultAPIURL
	}

	if !serverReachable(baseURL) {
		t.Skipf("server not reachable at %s — start it (go run ./cmd/app) or set LOYALTY_API_URL", baseURL)
	}

	c := &apiClient{baseURL: baseURL}

	if dbPath := os.Getenv("LOYALTY_DB_PATH"); dbPath != "" {
		dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&mode=ro", dbPath)
		db, err := sqlite.NewClient(context.Background(), dsn)
		if err != nil {
			t.Fatalf("open db at %s: %v", dbPath, err)
		}
		c.db = db
		t.Cleanup(func() { _ = db.Close() })
	}

	return c
}

// serverReachable reports whether an HTTP request to the endpoint gets any
// response (even a 405) — i.e. the server is listening.
func serverReachable(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// rpcResponse is a decoded JSON-RPC 2.0 response.
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call POSTs a single JSON-RPC request and returns the decoded response.
// params is the single params object (sent as the required one-element array);
// token, when non-empty, is sent as a Bearer credential.
func (c *apiClient) call(t *testing.T, method string, params any, token string) rpcResponse {
	t.Helper()

	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  []any{params},
		"id":      1,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var decoded rpcResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode response %q: %v", body, err)
	}
	return decoded
}

// uniqueEmail returns an address unlikely to collide with a previous run, since
// the tests run against a persistent server whose data outlives the process.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate unique email: %v", err)
	}
	return fmt.Sprintf("ada+%s@example.com", hex.EncodeToString(buf))
}
