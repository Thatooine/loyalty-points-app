// Package tests holds black-box JSON-RPC integration tests run over HTTP against
// an already-running server. The server URL is taken from LOYALTY_API_URL
// (default http://localhost:8080/api); the database is opened at the hardcoded
// testDBDSN. If the server is unreachable the tests skip rather than fail.
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

	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
)

const defaultAPIURL = "http://localhost:8080/api"

const testDBDSN = "postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable"

type apiClient struct {
	baseURL string
	db      *sql.DB // opened by setup at testDBDSN; the c.db == nil checks are defensive
}

// setup skips the test when the server is unreachable.
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

	if dsn := testDBDSN; dsn != "" {
		db, err := postgres.NewClient(context.Background(), dsn)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		c.db = db
		t.Cleanup(func() { _ = db.Close() })
	}

	return c
}

func serverReachable(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// params is the single params object, sent as the required one-element array.
func (c *apiClient) call(t *testing.T, method string, params any, token string) rpcResponse {
	t.Helper()
	resp, err := c.callRaw(method, params, token)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return resp
}

// callRaw is the goroutine-safe core of call: it touches no *testing.T, so it
// is safe to invoke from concurrently-spawned goroutines (where t.Fatalf is
// forbidden). Concurrency tests collect its (rpcResponse, error) and assert on
// the test goroutine after joining.
func (c *apiClient) callRaw(method string, params any, token string) (rpcResponse, error) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  []any{params},
		"id":      1,
	})
	if err != nil {
		return rpcResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return rpcResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("read response: %w", err)
	}

	var decoded rpcResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return rpcResponse{}, fmt.Errorf("decode response %q: %w", body, err)
	}
	return decoded, nil
}

// The server is persistent across runs, so each test must mint a fresh email.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate unique email: %v", err)
	}
	return fmt.Sprintf("ada+%s@example.com", hex.EncodeToString(buf))
}
