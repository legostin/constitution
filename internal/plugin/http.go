package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// HTTPPlugin calls an HTTP endpoint for check execution.
type HTTPPlugin struct {
	name    string
	url     string
	timeout time.Duration
	headers map[string]string
	client  *http.Client
}

// NewHTTPPlugin creates an HTTPPlugin from config.
func NewHTTPPlugin(cfg types.PluginConfig) (*HTTPPlugin, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("http plugin %q: url is required", cfg.Name)
	}
	timeout := 10 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Millisecond
	}

	headers := make(map[string]string)
	if cfg.Config != nil {
		if h, ok := cfg.Config["headers"].(map[string]interface{}); ok {
			for k, v := range h {
				if s, ok := v.(string); ok {
					headers[k] = s
				}
			}
		}
	}

	return &HTTPPlugin{
		name:    cfg.Name,
		url:     cfg.URL,
		timeout: timeout,
		headers: headers,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (p *HTTPPlugin) Name() string { return p.name }

type httpPluginRequest struct {
	Input  *types.HookInput       `json:"input"`
	Params map[string]interface{} `json:"params"`
}

func (p *HTTPPlugin) Execute(ctx context.Context, input *types.HookInput, params map[string]interface{}) (*types.CheckResult, error) {
	payload := httpPluginRequest{Input: input, Params: params}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("http plugin %q: marshal error: %w", p.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("http plugin %q: request error: %w", p.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http plugin %q: request failed: %w", p.name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http plugin %q: read response failed: %w", p.name, err)
	}

	if resp.StatusCode >= 400 {
		return &types.CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("HTTP plugin returned %d: %s", resp.StatusCode, string(body)),
		}, nil
	}

	var result types.CheckResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("http plugin %q: unmarshal error: %w", p.name, err)
	}
	return &result, nil
}

func (p *HTTPPlugin) Close() error { return nil }
