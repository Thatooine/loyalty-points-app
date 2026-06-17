// Package tests holds black-box JSON-RPC integration tests run over HTTP against
// an already-running server, configured via LOYALTY_API_URL and LOYALTY_DB_DSN.
// If the server is unreachable the tests skip rather than fail.
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

type apiClient struct {
	baseURL string
	db      *sql.DB // non-nil only when LOYALTY_DB_DSN is set
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

	if dsn := "postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable"; dsn != "" {
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

// The server is persistent across runs, so each test must mint a fresh email.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate unique email: %v", err)
	}
	return fmt.Sprintf("ada+%s@example.com", hex.EncodeToString(buf))
}
