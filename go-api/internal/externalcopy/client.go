package externalcopy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CopyRequest is sent to the external compliant-copy REST API.
type CopyRequest struct {
	RequestID string `json:"request_id,omitempty"`

	DocumentID string `json:"document_id"`
	Filename   string `json:"filename"`
	MIMEType   string `json:"mime_type"`
	Checksum   string `json:"checksum"`
	StorageURI string `json:"storage_uri"`
	CreatedAt  string `json:"created_at"`
}

// CopyResult describes the external copy API response.
type CopyResult struct {
	StatusCode int
	Body       map[string]any
	Attempts   int
}

// CallError captures non-success copy API failures and retry details.
type CallError struct {
	StatusCode int
	Body       string
	Retriable  bool
	Attempts   int
	Cause      error
}

func (e *CallError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("copy request failed after %d attempt(s): %v", e.Attempts, e.Cause)
	}
	if e.StatusCode > 0 {
		if e.Body == "" {
			return fmt.Sprintf("copy request failed with status %d after %d attempt(s)", e.StatusCode, e.Attempts)
		}
		return fmt.Sprintf("copy request failed with status %d after %d attempt(s): %s", e.StatusCode, e.Attempts, e.Body)
	}
	return fmt.Sprintf("copy request failed after %d attempt(s)", e.Attempts)
}

func (e *CallError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type Client struct {
	baseURL     string
	token       string
	httpClient  *http.Client
	maxAttempts int
	backoff     time.Duration
}

func NewClient(baseURL, token string, timeout time.Duration, maxAttempts int) *Client {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxAttempts: maxAttempts,
		backoff:     250 * time.Millisecond,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) CopyDocument(ctx context.Context, req CopyRequest) (CopyResult, error) {
	if !c.Enabled() {
		return CopyResult{}, &CallError{Retriable: false, Attempts: 0, Cause: errors.New("external copy API is disabled")}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return CopyResult{}, fmt.Errorf("marshal copy request: %w", err)
	}

	var lastErr *CallError
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/copies", bytes.NewReader(body))
		if err != nil {
			return CopyResult{}, fmt.Errorf("build request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if c.token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, doErr := c.httpClient.Do(httpReq)
		if doErr != nil {
			callErr := &CallError{Retriable: true, Attempts: attempt, Cause: doErr}
			lastErr = callErr
			if attempt == c.maxAttempts || !sleepWithContext(ctx, c.backoff*time.Duration(attempt)) {
				return CopyResult{}, callErr
			}
			continue
		}

		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		_ = resp.Body.Close()
		payloadText := strings.TrimSpace(string(payload))

		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			parsed := map[string]any{}
			if len(payload) > 0 {
				_ = json.Unmarshal(payload, &parsed)
			}
			return CopyResult{StatusCode: resp.StatusCode, Body: parsed, Attempts: attempt}, nil
		}

		retriable := isRetriableStatus(resp.StatusCode)
		callErr := &CallError{StatusCode: resp.StatusCode, Body: payloadText, Retriable: retriable, Attempts: attempt}
		lastErr = callErr
		if !retriable || attempt == c.maxAttempts || !sleepWithContext(ctx, c.backoff*time.Duration(attempt)) {
			return CopyResult{}, callErr
		}
	}

	if lastErr != nil {
		return CopyResult{}, lastErr
	}
	return CopyResult{}, &CallError{Retriable: true, Attempts: c.maxAttempts}
}

func isRetriableStatus(status int) bool {
	if status == http.StatusRequestTimeout || status == http.StatusTooManyRequests {
		return true
	}
	return status >= 500 && status <= 599
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
