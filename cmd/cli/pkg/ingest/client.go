package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Response is one element of a JSON-RPC 2.0 batch response. Exactly one of
// Result / Error is set per the spec.
type Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

// RPCError is the JSON-RPC error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Send POSTs the batch to the JSON-RPC endpoint with an optional bearer token
// and decodes the per-element responses.
func Send(ctx context.Context, client *http.Client, url, token string, batch []Request) ([]Response, error) {
	body, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("could not marshal batch: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}

	var responses []Response
	if err := json.Unmarshal(respBody, &responses); err != nil {
		// Not a batch array — surface the raw body (e.g. a single error
		// envelope from a transport-level rejection).
		return nil, fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return responses, nil
}
