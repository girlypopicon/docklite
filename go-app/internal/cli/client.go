package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	Timeout time.Duration
}

// APIError is a response from the agent with an error status. Callers use
// Status to tell "not allowed" from "not found" from "bad request".
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("request failed (HTTP %d)", e.Status)
	}
	return e.Message
}

// UnreachableError means no HTTP response at all: wrong host or port, the
// agent isn't running, or the network is down.
type UnreachableError struct {
	URL string
	Err error
}

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("cannot reach DockLite at %s: %v", e.URL, e.Err)
}

func (e *UnreachableError) Unwrap() error { return e.Err }

// errorMessage pulls {"error": "..."} out of an error body, else the raw text.
func errorMessage(body []byte) string {
	var parsed struct {
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != "" {
		if parsed.Detail != "" {
			return parsed.Error + ": " + strings.TrimSpace(parsed.Detail)
		}
		return parsed.Error
	}
	return strings.TrimSpace(string(body))
}

func (c *Client) Do(ctx context.Context, method string, path string, payload any) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("client is not initialized")
	}
	url := strings.TrimRight(c.BaseURL, "/") + path

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	httpClient := &http.Client{Timeout: c.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &UnreachableError{URL: strings.TrimRight(c.BaseURL, "/"), Err: err}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Message: errorMessage(data)}
	}
	return data, nil
}
