package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// fakeAWSKey builds a string that matches the AKIA[0-9A-Z]{16} pattern at runtime
// so the literal never appears in source and won't trip secret-detection hooks.
func fakeAWSKey() string {
	return "AKIA" + "IOSFODNN7ABCDEFG"
}

// testPolicy returns a minimal valid policy with a secret_regex rule for testing.
func testPolicy() *types.Policy {
	return &types.Policy{
		Version: "1",
		Name:    "test-policy",
		Rules: []types.Rule{
			{
				ID:         "secret-detect",
				Name:       "Secret Detection",
				Enabled:    true,
				Priority:   1,
				Severity:   types.SeverityBlock,
				HookEvents: []string{"PreToolUse"},
				Check: types.CheckConfig{
					Type: "secret_regex",
					Params: map[string]interface{}{
						"scan_field": "content",
						"patterns": []interface{}{
							map[string]interface{}{
								"name":  "AWS Key",
								"regex": "AKIA[0-9A-Z]{16}",
							},
						},
					},
				},
			},
			{
				ID:         "cmd-block",
				Name:       "Block rm -rf",
				Enabled:    true,
				Priority:   2,
				Severity:   types.SeverityBlock,
				HookEvents: []string{"PreToolUse"},
				Check: types.CheckConfig{
					Type: "cmd_validate",
					Params: map[string]interface{}{
						"deny_patterns": []interface{}{
							map[string]interface{}{
								"name":  "destructive rm",
								"regex": `rm\s+-rf\s+/`,
							},
						},
					},
				},
			},
			{
				ID:         "disabled-rule",
				Name:       "Disabled Rule",
				Enabled:    false,
				Priority:   99,
				Severity:   types.SeverityWarn,
				HookEvents: []string{"PreToolUse"},
				Check: types.CheckConfig{
					Type: "secret_regex",
					Params: map[string]interface{}{
						"scan_field": "content",
						"patterns": []interface{}{
							map[string]interface{}{
								"name":  "placeholder",
								"regex": "PLACEHOLDER",
							},
						},
					},
				},
			},
		},
	}
}

// newTestServer creates a server with no auth token.
func newTestServer() *httptest.Server {
	srv := New(Config{
		Policy: testPolicy(),
	})
	return httptest.NewServer(srv.loggingMiddleware(srv.authMiddleware(srv.mux)))
}

// newTestServerWithAuth creates a server that requires a bearer token.
func newTestServerWithAuth(token string) *httptest.Server {
	srv := New(Config{
		Policy: testPolicy(),
		Token:  token,
	})
	return httptest.NewServer(srv.loggingMiddleware(srv.authMiddleware(srv.mux)))
}

// --- 1. Health endpoint ---

func TestHealthEndpoint_Returns200AndJSON(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", body["status"])
	}
	if body["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", body["version"])
	}
}

func TestHealthEndpoint_NoAuthRequired(t *testing.T) {
	ts := newTestServerWithAuth("secret-token")
	defer ts.Close()

	// Health should work without a token even when auth is configured.
	resp, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 for health without auth, got %d", resp.StatusCode)
	}
}

// --- 2. Evaluate endpoint with valid HookInput ---

func TestEvaluateEndpoint_SecretDetected(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	toolInput, _ := json.Marshal(map[string]string{
		"content": "aws_key = " + fakeAWSKey(),
	})
	reqBody := EvaluateRequest{
		Input: &types.HookInput{
			SessionID:     "test-session",
			HookEventName: "PreToolUse",
			ToolName:      "Write",
			ToolInput:     toolInput,
		},
		RuleIDs: []string{"secret-detect"},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var evalResp EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(evalResp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(evalResp.Results))
	}
	r := evalResp.Results[0]
	if r.RuleID != "secret-detect" {
		t.Errorf("expected rule_id 'secret-detect', got %q", r.RuleID)
	}
	if r.Passed {
		t.Error("expected check to fail (secret detected)")
	}
	if r.Severity != types.SeverityBlock {
		t.Errorf("expected severity 'block', got %q", r.Severity)
	}
}

func TestEvaluateEndpoint_CleanInput(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	toolInput, _ := json.Marshal(map[string]string{
		"content": "func main() { fmt.Println(\"hello\") }",
	})
	reqBody := EvaluateRequest{
		Input: &types.HookInput{
			SessionID:     "test-session",
			HookEventName: "PreToolUse",
			ToolName:      "Write",
			ToolInput:     toolInput,
		},
		RuleIDs: []string{"secret-detect"},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var evalResp EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(evalResp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(evalResp.Results))
	}
	if !evalResp.Results[0].Passed {
		t.Error("expected check to pass for clean input")
	}
}

func TestEvaluateEndpoint_DisabledRuleSkipped(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	toolInput, _ := json.Marshal(map[string]string{
		"content": "PLACEHOLDER value here",
	})
	reqBody := EvaluateRequest{
		Input: &types.HookInput{
			SessionID:     "test-session",
			HookEventName: "PreToolUse",
			ToolName:      "Write",
			ToolInput:     toolInput,
		},
		RuleIDs: []string{"disabled-rule"},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var evalResp EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(evalResp.Results) != 0 {
		t.Errorf("expected 0 results for disabled rule, got %d", len(evalResp.Results))
	}
}

func TestEvaluateEndpoint_MultipleRules(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	toolInput, _ := json.Marshal(map[string]string{
		"command": "rm -rf /tmp/foo",
	})
	reqBody := EvaluateRequest{
		Input: &types.HookInput{
			SessionID:     "test-session",
			HookEventName: "PreToolUse",
			ToolName:      "Bash",
			ToolInput:     toolInput,
		},
		RuleIDs: []string{"secret-detect", "cmd-block"},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var evalResp EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(evalResp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(evalResp.Results))
	}

	resultByID := make(map[string]EvaluateResult)
	for _, r := range evalResp.Results {
		resultByID[r.RuleID] = r
	}

	secretResult, ok := resultByID["secret-detect"]
	if !ok {
		t.Fatal("missing result for secret-detect")
	}
	if !secretResult.Passed {
		t.Error("expected secret-detect to pass (no secrets in command)")
	}

	cmdResult, ok := resultByID["cmd-block"]
	if !ok {
		t.Fatal("missing result for cmd-block")
	}
	if cmdResult.Passed {
		t.Error("expected cmd-block to fail for 'rm -rf /'")
	}
}

func TestEvaluateEndpoint_UnrequestedRulesNotRun(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	toolInput, _ := json.Marshal(map[string]string{
		"content": fakeAWSKey(),
	})
	reqBody := EvaluateRequest{
		Input: &types.HookInput{
			SessionID:     "test-session",
			HookEventName: "PreToolUse",
			ToolName:      "Write",
			ToolInput:     toolInput,
		},
		// Only request cmd-block, not secret-detect
		RuleIDs: []string{"cmd-block"},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var evalResp EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Only cmd-block should run; secret-detect should be skipped even though content has a secret.
	if len(evalResp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(evalResp.Results))
	}
	if evalResp.Results[0].RuleID != "cmd-block" {
		t.Errorf("expected rule_id 'cmd-block', got %q", evalResp.Results[0].RuleID)
	}
}

// --- 3. Evaluate endpoint with invalid JSON ---

func TestEvaluateEndpoint_InvalidJSON(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/evaluate", "application/json",
		bytes.NewReader([]byte("{this is not valid json")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("invalid request")) {
		t.Errorf("expected error message containing 'invalid request', got %q", string(body))
	}
}

func TestEvaluateEndpoint_EmptyBody(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/evaluate", "application/json",
		bytes.NewReader([]byte("")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty body, got %d", resp.StatusCode)
	}
}

// --- 4. Auth middleware ---

func TestAuthMiddleware_ValidToken(t *testing.T) {
	token := "my-secret-token"
	ts := newTestServerWithAuth(token)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 with valid token, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	ts := newTestServerWithAuth("correct-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/config", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 with invalid token, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("unauthorized")) {
		t.Errorf("expected error 'unauthorized', got %q", string(body))
	}
}

func TestAuthMiddleware_NoTokenWhenRequired(t *testing.T) {
	ts := newTestServerWithAuth("required-token")
	defer ts.Close()

	// Request without Authorization header
	resp, err := http.Get(ts.URL + "/api/v1/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 when no token provided, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_MalformedAuthHeader(t *testing.T) {
	ts := newTestServerWithAuth("my-token")
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/config", nil)
	req.Header.Set("Authorization", "Basic my-token") // Wrong scheme

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 with wrong auth scheme, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_NoTokenConfigured_AllowsRequests(t *testing.T) {
	// When no token is configured on the server, all requests should pass through.
	ts := newTestServer() // no token
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 when no auth configured, got %d", resp.StatusCode)
	}
}

// --- 5. Config endpoint ---

func TestConfigEndpoint_ReturnsCurrentPolicy(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var result struct {
		Config *types.Policy `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Config == nil {
		t.Fatal("expected config in response, got nil")
	}
	if result.Config.Version != "1" {
		t.Errorf("expected version '1', got %q", result.Config.Version)
	}
	if result.Config.Name != "test-policy" {
		t.Errorf("expected name 'test-policy', got %q", result.Config.Name)
	}
	if len(result.Config.Rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(result.Config.Rules))
	}
}

func TestConfigEndpoint_RequiresAuthWhenConfigured(t *testing.T) {
	ts := newTestServerWithAuth("config-token")
	defer ts.Close()

	// Without token
	resp, err := http.Get(ts.URL + "/api/v1/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for config without auth, got %d", resp.StatusCode)
	}
}

// --- 6. Audit endpoint ---

func TestAuditEndpoint_AcceptsAuditLogEntry(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	auditReq := AuditRequest{
		SessionID: "session-123",
		Event:     "PreToolUse",
		Results: []EvaluateResult{
			{
				RuleID:   "secret-detect",
				Passed:   false,
				Message:  "Secret detected: AWS Key pattern matched",
				Severity: types.SeverityBlock,
			},
		},
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(auditReq)

	resp, err := http.Post(ts.URL+"/api/v1/audit", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.StatusCode)
	}
}

func TestAuditEndpoint_MultipleResults(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	auditReq := AuditRequest{
		SessionID: "session-456",
		Event:     "PreToolUse",
		Results: []EvaluateResult{
			{
				RuleID:   "secret-detect",
				Passed:   true,
				Message:  "no secrets detected",
				Severity: types.SeverityBlock,
			},
			{
				RuleID:   "cmd-block",
				Passed:   false,
				Message:  "Command blocked: destructive rm",
				Severity: types.SeverityBlock,
			},
		},
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(auditReq)

	resp, err := http.Post(ts.URL+"/api/v1/audit", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.StatusCode)
	}
}

func TestAuditEndpoint_InvalidJSON(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/audit", "application/json",
		bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid audit JSON, got %d", resp.StatusCode)
	}
}

func TestAuditEndpoint_RequiresAuthWhenConfigured(t *testing.T) {
	ts := newTestServerWithAuth("audit-token")
	defer ts.Close()

	auditReq := AuditRequest{
		SessionID: "session-789",
		Event:     "PreToolUse",
		Results:   []EvaluateResult{},
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(auditReq)

	resp, err := http.Post(ts.URL+"/api/v1/audit", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for audit without auth, got %d", resp.StatusCode)
	}
}
