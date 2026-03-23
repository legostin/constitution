package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// Client communicates with the constitutiond remote service.
type Client struct {
	url     string
	token   string
	timeout time.Duration
	client  *http.Client
}

// NewClient creates a Client from RemoteConfig.
func NewClient(cfg types.RemoteConfig) *Client {
	token := cfg.AuthToken
	if token == "" && cfg.AuthTokenEnv != "" {
		token = os.Getenv(cfg.AuthTokenEnv)
	}
	timeout := 5 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Millisecond
	}
	return &Client{
		url:     cfg.URL,
		token:   token,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

// EvaluateRequest is sent to the remote /api/v1/evaluate endpoint.
type EvaluateRequest struct {
	Input   *types.HookInput `json:"input"`
	RuleIDs []string         `json:"rule_ids"`
}

// EvaluateResult is one rule result from the remote service.
type EvaluateResult struct {
	RuleID   string         `json:"rule_id"`
	Passed   bool           `json:"passed"`
	Message  string         `json:"message"`
	Severity types.Severity `json:"severity"`
}

// EvaluateResponse is the response from /api/v1/evaluate.
type EvaluateResponse struct {
	Results []EvaluateResult `json:"results"`
}

// Evaluate sends rules for remote evaluation.
func (c *Client) Evaluate(ctx context.Context, input *types.HookInput, ruleIDs []string) (*EvaluateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := EvaluateRequest{Input: input, RuleIDs: ruleIDs}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("remote: marshal error: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/v1/evaluate", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("remote: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("remote: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("remote: read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote: server returned %d: %s", resp.StatusCode, string(body))
	}

	var evalResp EvaluateResponse
	if err := json.Unmarshal(body, &evalResp); err != nil {
		return nil, fmt.Errorf("remote: unmarshal error: %w", err)
	}
	return &evalResp, nil
}

// AuditRequest is sent to /api/v1/audit for logging.
type AuditRequest struct {
	SessionID string              `json:"session_id"`
	Event     string              `json:"event"`
	Results   []EvaluateResult    `json:"results"`
	Timestamp time.Time           `json:"timestamp"`
}

// Audit sends audit log entries to the remote service (fire-and-forget).
func (c *Client) Audit(ctx context.Context, sessionID, event string, results []EvaluateResult) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req := AuditRequest{
		SessionID: sessionID,
		Event:     event,
		Results:   results,
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/v1/audit", bytes.NewReader(data))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// Health checks the remote service health.
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/api/v1/health", nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote: health check returned %d", resp.StatusCode)
	}
	return nil
}
