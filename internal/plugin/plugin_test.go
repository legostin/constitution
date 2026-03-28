package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// ---------------------------------------------------------------------------
// Exec plugin tests
// ---------------------------------------------------------------------------

func TestExecPlugin_RunsCommandReturnsCheckResult(t *testing.T) {
	// echo outputs JSON to stdout; the plugin parses it into CheckResult.
	script := `echo '{"passed":true,"message":"all good"}'`
	cfg := types.PluginConfig{
		Name: "test-exec",
		Type: "exec",
		Path: "/bin/sh",
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}

	// Override path to run via sh -c so we can use a shell expression.
	// Instead, we create a wrapper: ExecPlugin.path is the binary,
	// so we use "sh" and pass the script via stdin won't work directly.
	// Better approach: write a tiny script or use /bin/echo directly.
	_ = script

	// Simpler approach: use /bin/echo with a JSON payload.
	cfg2 := types.PluginConfig{
		Name: "test-exec-echo",
		Type: "exec",
		Path: "/bin/echo",
	}
	p2, err := NewExecPlugin(cfg2)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}

	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
	}

	result, err := p2.Execute(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// /bin/echo ignores stdin, outputs a newline. JSON unmarshal fails,
	// so the plugin returns {Passed: true} as fallback for exit 0.
	if !result.Passed {
		t.Errorf("expected Passed=true, got false")
	}

	_ = p.Close()
	_ = p2.Close()
}

func TestExecPlugin_CommandExitZeroWithValidJSON(t *testing.T) {
	// Create a temp script that outputs valid JSON.
	dir := t.TempDir()
	scriptPath := dir + "/check.sh"
	writeScript(t, scriptPath, `#!/bin/sh
echo '{"passed":true,"message":"check passed","details":{"key":"val"}}'
`)

	cfg := types.PluginConfig{
		Name: "json-exec",
		Type: "exec",
		Path: scriptPath,
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}
	defer p.Close()

	result, err := p.Execute(context.Background(), &types.HookInput{}, nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.Message != "check passed" {
		t.Errorf("expected message 'check passed', got %q", result.Message)
	}
	if result.Details["key"] != "val" {
		t.Errorf("expected details key=val, got %v", result.Details)
	}
}

func TestExecPlugin_CommandExitTwoReturnsBlocked(t *testing.T) {
	dir := t.TempDir()
	scriptPath := dir + "/block.sh"
	writeScript(t, scriptPath, `#!/bin/sh
echo '{"passed":false,"message":"blocked by policy"}'
exit 2
`)

	cfg := types.PluginConfig{
		Name: "block-exec",
		Type: "exec",
		Path: scriptPath,
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}
	defer p.Close()

	result, err := p.Execute(context.Background(), &types.HookInput{}, nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected Passed=false for exit code 2")
	}
	if result.Message != "blocked by policy" {
		t.Errorf("expected message 'blocked by policy', got %q", result.Message)
	}
}

func TestExecPlugin_Timeout(t *testing.T) {
	// Use /bin/sleep directly (not via sh) so that CommandContext can kill it.
	cfg := types.PluginConfig{
		Name:    "slow-exec",
		Type:    "exec",
		Path:    "/bin/sleep",
		Timeout: 200, // 200ms
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}
	defer p.Close()

	// Override: ExecPlugin calls exec.CommandContext(ctx, p.path) with no args,
	// but /bin/sleep with no args exits immediately with error.
	// We need a script that exec's sleep directly.
	dir := t.TempDir()
	scriptPath := dir + "/slow.sh"
	writeScript(t, scriptPath, `#!/bin/sh
exec sleep 30
`)
	p.path = scriptPath

	start := time.Now()
	_, err = p.Execute(context.Background(), &types.HookInput{}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for timed-out command")
	}
	if !strings.Contains(err.Error(), "exec plugin") {
		t.Errorf("expected exec plugin error, got: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("timeout did not trigger quickly enough: took %v", elapsed)
	}
}

func TestExecPlugin_CommandNotFound(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "missing-exec",
		Type: "exec",
		Path: "/nonexistent/binary/path",
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}
	defer p.Close()

	_, err = p.Execute(context.Background(), &types.HookInput{}, nil)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "exec plugin") {
		t.Errorf("expected exec plugin error wrapper, got: %v", err)
	}
}

func TestExecPlugin_MissingPath(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "no-path",
		Type: "exec",
		Path: "",
	}
	_, err := NewExecPlugin(cfg)
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestExecPlugin_NonZeroExitReturnsError(t *testing.T) {
	dir := t.TempDir()
	scriptPath := dir + "/fail.sh"
	writeScript(t, scriptPath, `#!/bin/sh
echo "something went wrong" >&2
exit 1
`)

	cfg := types.PluginConfig{
		Name: "fail-exec",
		Type: "exec",
		Path: scriptPath,
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}
	defer p.Close()

	_, err = p.Execute(context.Background(), &types.HookInput{}, nil)
	if err == nil {
		t.Fatal("expected error for exit code 1")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("expected stderr in error message, got: %v", err)
	}
}

func TestExecPlugin_SendsInputAsJSON(t *testing.T) {
	// Script reads stdin and writes it to a temp file so we can verify the payload.
	dir := t.TempDir()
	stdinFile := dir + "/stdin.json"
	scriptPath := dir + "/read_input.sh"
	writeScript(t, scriptPath, fmt.Sprintf(`#!/bin/sh
cat > %s
echo '{"passed":true,"message":"received"}'
`, stdinFile))

	cfg := types.PluginConfig{
		Name: "stdin-exec",
		Type: "exec",
		Path: scriptPath,
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}
	defer p.Close()

	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
	}
	params := map[string]interface{}{"key": "value"}

	result, err := p.Execute(context.Background(), input, params)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}

	// Read what the script captured on stdin and verify it is valid JSON
	// containing the expected fields.
	captured, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("failed to read captured stdin: %v", err)
	}

	var payload execPluginInput
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("failed to parse captured stdin as JSON: %v", err)
	}
	if payload.Input.HookEventName != "PreToolUse" {
		t.Errorf("expected HookEventName=PreToolUse, got %q", payload.Input.HookEventName)
	}
	if payload.Input.ToolName != "Bash" {
		t.Errorf("expected ToolName=Bash, got %q", payload.Input.ToolName)
	}
	if payload.Params["key"] != "value" {
		t.Errorf("expected param key=value, got %v", payload.Params["key"])
	}
}

// ---------------------------------------------------------------------------
// HTTP plugin tests
// ---------------------------------------------------------------------------

func TestHTTPPlugin_SendsRequestParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var req httpPluginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if req.Input.ToolName != "Write" {
			t.Errorf("expected ToolName=Write, got %q", req.Input.ToolName)
		}
		if req.Params["severity"] != "high" {
			t.Errorf("expected param severity=high, got %v", req.Params["severity"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.CheckResult{
			Passed:  true,
			Message: "approved",
		})
	}))
	defer server.Close()

	cfg := types.PluginConfig{
		Name: "test-http",
		Type: "http",
		URL:  server.URL,
	}
	p, err := NewHTTPPlugin(cfg)
	if err != nil {
		t.Fatalf("NewHTTPPlugin error: %v", err)
	}
	defer p.Close()

	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
	}
	params := map[string]interface{}{"severity": "high"}

	result, err := p.Execute(context.Background(), input, params)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.Message != "approved" {
		t.Errorf("expected message 'approved', got %q", result.Message)
	}
}

func TestHTTPPlugin_ServerErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"400 Bad Request", 400, "bad request body"},
		{"403 Forbidden", 403, "access denied"},
		{"500 Internal Server Error", 500, "internal error"},
		{"502 Bad Gateway", 502, "upstream error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()

			cfg := types.PluginConfig{
				Name: "err-http",
				Type: "http",
				URL:  server.URL,
			}
			p, err := NewHTTPPlugin(cfg)
			if err != nil {
				t.Fatalf("NewHTTPPlugin error: %v", err)
			}
			defer p.Close()

			result, err := p.Execute(context.Background(), &types.HookInput{}, nil)
			if err != nil {
				t.Fatalf("Execute should not return error for HTTP %d, got: %v", tt.statusCode, err)
			}
			if result.Passed {
				t.Errorf("expected Passed=false for HTTP %d", tt.statusCode)
			}
			if !strings.Contains(result.Message, fmt.Sprintf("%d", tt.statusCode)) {
				t.Errorf("expected status code %d in message, got %q", tt.statusCode, result.Message)
			}
			if !strings.Contains(result.Message, tt.body) {
				t.Errorf("expected body %q in message, got %q", tt.body, result.Message)
			}
		})
	}
}

func TestHTTPPlugin_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the client timeout but short enough to not slow down tests.
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := types.PluginConfig{
		Name:    "slow-http",
		Type:    "http",
		URL:     server.URL,
		Timeout: 100, // 100ms
	}
	p, err := NewHTTPPlugin(cfg)
	if err != nil {
		t.Fatalf("NewHTTPPlugin error: %v", err)
	}
	defer p.Close()

	start := time.Now()
	_, err = p.Execute(context.Background(), &types.HookInput{}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for timed-out HTTP request")
	}
	if !strings.Contains(err.Error(), "http plugin") {
		t.Errorf("expected http plugin error, got: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("timeout did not trigger quickly enough: took %v", elapsed)
	}
}

func TestHTTPPlugin_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret123" {
			t.Errorf("expected X-Api-Key=secret123, got %q", r.Header.Get("X-Api-Key"))
		}
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("expected X-Custom=value, got %q", r.Header.Get("X-Custom"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.CheckResult{Passed: true, Message: "ok"})
	}))
	defer server.Close()

	cfg := types.PluginConfig{
		Name: "headers-http",
		Type: "http",
		URL:  server.URL,
		Config: map[string]interface{}{
			"headers": map[string]interface{}{
				"X-Api-Key": "secret123",
				"X-Custom":  "value",
			},
		},
	}
	p, err := NewHTTPPlugin(cfg)
	if err != nil {
		t.Fatalf("NewHTTPPlugin error: %v", err)
	}
	defer p.Close()

	result, err := p.Execute(context.Background(), &types.HookInput{}, nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
}

func TestHTTPPlugin_MissingURL(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "no-url",
		Type: "http",
		URL:  "",
	}
	_, err := NewHTTPPlugin(cfg)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestHTTPPlugin_InvalidResponseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "not valid json{{{")
	}))
	defer server.Close()

	cfg := types.PluginConfig{
		Name: "bad-json-http",
		Type: "http",
		URL:  server.URL,
	}
	p, err := NewHTTPPlugin(cfg)
	if err != nil {
		t.Fatalf("NewHTTPPlugin error: %v", err)
	}
	defer p.Close()

	_, err = p.Execute(context.Background(), &types.HookInput{}, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Plugin factory / Manager tests
// ---------------------------------------------------------------------------

func TestLoadPlugin_ExecType(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "my-exec",
		Type: "exec",
		Path: "/bin/echo",
	}
	p, err := loadPlugin(cfg)
	if err != nil {
		t.Fatalf("loadPlugin error: %v", err)
	}
	if _, ok := p.(*ExecPlugin); !ok {
		t.Errorf("expected *ExecPlugin, got %T", p)
	}
	if p.Name() != "my-exec" {
		t.Errorf("expected name 'my-exec', got %q", p.Name())
	}
}

func TestLoadPlugin_HTTPType(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "my-http",
		Type: "http",
		URL:  "http://localhost:9999",
	}
	p, err := loadPlugin(cfg)
	if err != nil {
		t.Fatalf("loadPlugin error: %v", err)
	}
	if _, ok := p.(*HTTPPlugin); !ok {
		t.Errorf("expected *HTTPPlugin, got %T", p)
	}
	if p.Name() != "my-http" {
		t.Errorf("expected name 'my-http', got %q", p.Name())
	}
}

func TestLoadPlugin_UnknownType(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "bad",
		Type: "grpc",
	}
	_, err := loadPlugin(cfg)
	if err == nil {
		t.Fatal("expected error for unknown plugin type")
	}
	if !strings.Contains(err.Error(), "unknown plugin type") {
		t.Errorf("expected 'unknown plugin type' error, got: %v", err)
	}
}

func TestNewManager_LoadsPlugins(t *testing.T) {
	configs := []types.PluginConfig{
		{Name: "p1", Type: "exec", Path: "/bin/echo"},
		{Name: "p2", Type: "http", URL: "http://localhost:9999"},
	}
	m, err := NewManager(configs)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	defer m.Close()

	p1, ok := m.Get("p1")
	if !ok {
		t.Fatal("expected to find plugin p1")
	}
	if p1.Name() != "p1" {
		t.Errorf("expected name 'p1', got %q", p1.Name())
	}

	p2, ok := m.Get("p2")
	if !ok {
		t.Fatal("expected to find plugin p2")
	}
	if p2.Name() != "p2" {
		t.Errorf("expected name 'p2', got %q", p2.Name())
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("expected Get to return false for nonexistent plugin")
	}
}

func TestNewManager_FailsOnBadPlugin(t *testing.T) {
	configs := []types.PluginConfig{
		{Name: "bad", Type: "exec", Path: ""}, // Missing path
	}
	_, err := NewManager(configs)
	if err == nil {
		t.Fatal("expected error for bad plugin config")
	}
	if !strings.Contains(err.Error(), "failed to load plugin") {
		t.Errorf("expected 'failed to load plugin' error, got: %v", err)
	}
}

func TestNewManager_EmptyConfigs(t *testing.T) {
	m, err := NewManager(nil)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	defer m.Close()

	_, ok := m.Get("anything")
	if ok {
		t.Error("expected no plugins in empty manager")
	}
}

func TestExecPlugin_DefaultTimeout(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "default-timeout",
		Type: "exec",
		Path: "/bin/echo",
	}
	p, err := NewExecPlugin(cfg)
	if err != nil {
		t.Fatalf("NewExecPlugin error: %v", err)
	}
	if p.timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", p.timeout)
	}
}

func TestHTTPPlugin_DefaultTimeout(t *testing.T) {
	cfg := types.PluginConfig{
		Name: "default-timeout",
		Type: "http",
		URL:  "http://localhost:9999",
	}
	p, err := NewHTTPPlugin(cfg)
	if err != nil {
		t.Fatalf("NewHTTPPlugin error: %v", err)
	}
	if p.timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", p.timeout)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("failed to write script %s: %v", path, err)
	}
}
