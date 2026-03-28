package remote

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

func TestEvaluate_SendsCorrectRequestAndParsesResponse(t *testing.T) {
	var receivedBody EvaluateRequest
	var receivedContentType string

	wantResults := []EvaluateResult{
		{RuleID: "rule-1", Passed: true, Message: "ok", Severity: types.SeverityAudit},
		{RuleID: "rule-2", Passed: false, Message: "blocked", Severity: types.SeverityBlock},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/evaluate" {
			t.Errorf("expected path /api/v1/evaluate, got %s", r.URL.Path)
		}
		receivedContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Fatalf("unmarshalling request body: %v", err)
		}

		resp := EvaluateResponse{Results: wantResults}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(types.RemoteConfig{
		URL:     srv.URL,
		Timeout: 5000,
	})

	input := &types.HookInput{
		SessionID:     "sess-123",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
	}
	ruleIDs := []string{"rule-1", "rule-2"}

	got, err := client.Evaluate(context.Background(), input, ruleIDs)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	// Verify request was correct
	if receivedContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", receivedContentType)
	}
	if receivedBody.Input.SessionID != "sess-123" {
		t.Errorf("request input.session_id = %q, want sess-123", receivedBody.Input.SessionID)
	}
	if receivedBody.Input.HookEventName != "PreToolUse" {
		t.Errorf("request input.hook_event_name = %q, want PreToolUse", receivedBody.Input.HookEventName)
	}
	if receivedBody.Input.ToolName != "Bash" {
		t.Errorf("request input.tool_name = %q, want Bash", receivedBody.Input.ToolName)
	}
	if len(receivedBody.RuleIDs) != 2 || receivedBody.RuleIDs[0] != "rule-1" || receivedBody.RuleIDs[1] != "rule-2" {
		t.Errorf("request rule_ids = %v, want [rule-1 rule-2]", receivedBody.RuleIDs)
	}

	// Verify response was parsed correctly
	if len(got.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(got.Results))
	}
	if got.Results[0].RuleID != "rule-1" || !got.Results[0].Passed {
		t.Errorf("results[0] = %+v, want rule-1/passed=true", got.Results[0])
	}
	if got.Results[1].RuleID != "rule-2" || got.Results[1].Passed {
		t.Errorf("results[1] = %+v, want rule-2/passed=false", got.Results[1])
	}
	if got.Results[1].Severity != types.SeverityBlock {
		t.Errorf("results[1].Severity = %q, want %q", got.Results[1].Severity, types.SeverityBlock)
	}
}

func TestEvaluate_TimeoutHandling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server that takes longer than the client timeout
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(EvaluateResponse{})
	}))
	defer srv.Close()

	client := NewClient(types.RemoteConfig{
		URL:     srv.URL,
		Timeout: 50, // 50ms -- much less than the 500ms server delay
	})

	input := &types.HookInput{SessionID: "sess-timeout"}

	_, err := client.Evaluate(context.Background(), input, []string{"r1"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// The error should indicate a timeout or context deadline exceeded
	errMsg := err.Error()
	if !strings.Contains(errMsg, "deadline exceeded") &&
		!strings.Contains(errMsg, "Timeout") &&
		!strings.Contains(errMsg, "timeout") {
		t.Errorf("error %q does not indicate timeout", errMsg)
	}
}

func TestEvaluate_AuthHeaderSentWhenTokenSet(t *testing.T) {
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(EvaluateResponse{})
	}))
	defer srv.Close()

	t.Run("with token", func(t *testing.T) {
		receivedAuth = ""
		client := NewClient(types.RemoteConfig{
			URL:       srv.URL,
			AuthToken: "secret-token-42",
			Timeout:   5000,
		})

		_, err := client.Evaluate(context.Background(), &types.HookInput{}, nil)
		if err != nil {
			t.Fatalf("Evaluate error: %v", err)
		}
		if receivedAuth != "Bearer secret-token-42" {
			t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer secret-token-42")
		}
	})

	t.Run("with token from env", func(t *testing.T) {
		receivedAuth = ""
		t.Setenv("TEST_CONSTITUTION_TOKEN", "env-token-99")
		client := NewClient(types.RemoteConfig{
			URL:          srv.URL,
			AuthTokenEnv: "TEST_CONSTITUTION_TOKEN",
			Timeout:      5000,
		})

		_, err := client.Evaluate(context.Background(), &types.HookInput{}, nil)
		if err != nil {
			t.Fatalf("Evaluate error: %v", err)
		}
		if receivedAuth != "Bearer env-token-99" {
			t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer env-token-99")
		}
	})

	t.Run("without token", func(t *testing.T) {
		receivedAuth = ""
		client := NewClient(types.RemoteConfig{
			URL:     srv.URL,
			Timeout: 5000,
		})

		_, err := client.Evaluate(context.Background(), &types.HookInput{}, nil)
		if err != nil {
			t.Fatalf("Evaluate error: %v", err)
		}
		if receivedAuth != "" {
			t.Errorf("Authorization = %q, want empty (no token configured)", receivedAuth)
		}
	})
}

func TestEvaluate_NetworkErrorHandling(t *testing.T) {
	// Use a URL that will refuse connections (closed server)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close() // Close immediately so connections are refused

	client := NewClient(types.RemoteConfig{
		URL:     srvURL,
		Timeout: 2000,
	})

	_, err := client.Evaluate(context.Background(), &types.HookInput{}, []string{"r1"})
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "remote: request failed") {
		t.Errorf("error %q does not contain expected prefix", err.Error())
	}
}

func TestEvaluate_InvalidResponseBody(t *testing.T) {
	t.Run("non-JSON response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("this is not json"))
		}))
		defer srv.Close()

		client := NewClient(types.RemoteConfig{URL: srv.URL, Timeout: 5000})

		_, err := client.Evaluate(context.Background(), &types.HookInput{}, nil)
		if err == nil {
			t.Fatal("expected unmarshal error, got nil")
		}
		if !strings.Contains(err.Error(), "remote: unmarshal error") {
			t.Errorf("error %q does not contain 'remote: unmarshal error'", err.Error())
		}
	})

	t.Run("non-200 status code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}))
		defer srv.Close()

		client := NewClient(types.RemoteConfig{URL: srv.URL, Timeout: 5000})

		_, err := client.Evaluate(context.Background(), &types.HookInput{}, nil)
		if err == nil {
			t.Fatal("expected error for non-200, got nil")
		}
		if !strings.Contains(err.Error(), "remote: server returned 500") {
			t.Errorf("error %q does not contain 'remote: server returned 500'", err.Error())
		}
	})

	t.Run("malformed JSON structure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// Valid JSON but broken mid-stream
			w.Write([]byte(`{"results": [`))
		}))
		defer srv.Close()

		client := NewClient(types.RemoteConfig{URL: srv.URL, Timeout: 5000})

		_, err := client.Evaluate(context.Background(), &types.HookInput{}, nil)
		if err == nil {
			t.Fatal("expected unmarshal error, got nil")
		}
		if !strings.Contains(err.Error(), "remote: unmarshal error") {
			t.Errorf("error %q does not contain 'remote: unmarshal error'", err.Error())
		}
	})
}
