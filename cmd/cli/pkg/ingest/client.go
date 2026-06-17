package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Response is a JSON-RPC 2.0 response. Exactly one of Result / Error is set.
type Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Send(ctx context.Context, client *http.Client, url, token string, request Request) (Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return Response{}, fmt.Errorf("could not marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("could not build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("could not reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("could not read response: %w", err)
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		return Response{}, fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return response, nil
}
