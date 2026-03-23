package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/legostin/constitution/internal/server/store"
	"github.com/legostin/constitution/pkg/types"
)

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("POST /api/v1/evaluate", s.handleEvaluate)
	s.mux.HandleFunc("POST /api/v1/audit", s.handleAudit)
	s.mux.HandleFunc("GET /api/v1/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
}

// EvaluateRequest from the local binary.
type EvaluateRequest struct {
	Input   *types.HookInput `json:"input"`
	RuleIDs []string         `json:"rule_ids"`
}

// EvaluateResult for one rule.
type EvaluateResult struct {
	RuleID   string         `json:"rule_id"`
	Passed   bool           `json:"passed"`
	Message  string         `json:"message"`
	Severity types.Severity `json:"severity"`
}

// EvaluateResponse sent back to the local binary.
type EvaluateResponse struct {
	Results []EvaluateResult `json:"results"`
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Build rule ID set for fast lookup
	ruleIDSet := make(map[string]bool)
	for _, id := range req.RuleIDs {
		ruleIDSet[id] = true
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	var results []EvaluateResult

	for _, rule := range s.policy.Rules {
		if !ruleIDSet[rule.ID] {
			continue
		}
		if !rule.Enabled {
			continue
		}

		c, err := s.registry.Get(rule.Check.Type)
		if err != nil {
			slog.Warn("unknown check type", "type", rule.Check.Type, "rule", rule.ID)
			continue
		}
		if err := c.Init(rule.Check.Params); err != nil {
			slog.Error("check init failed", "rule", rule.ID, "error", err)
			continue
		}

		result, err := c.Execute(ctx, req.Input)
		if err != nil {
			slog.Error("check execution failed", "rule", rule.ID, "error", err)
			continue
		}

		evalResult := EvaluateResult{
			RuleID:   rule.ID,
			Passed:   result.Passed,
			Message:  result.Message,
			Severity: rule.Severity,
		}
		results = append(results, evalResult)

		// Audit log
		s.store.SaveAudit(store.AuditEntry{
			SessionID: req.Input.SessionID,
			Event:     req.Input.HookEventName,
			RuleID:    rule.ID,
			Passed:    result.Passed,
			Message:   result.Message,
			Severity:  string(rule.Severity),
			Timestamp: time.Now(),
		})
	}

	resp := EvaluateResponse{Results: results}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// AuditRequest from the local binary.
type AuditRequest struct {
	SessionID string           `json:"session_id"`
	Event     string           `json:"event"`
	Results   []EvaluateResult `json:"results"`
	Timestamp time.Time        `json:"timestamp"`
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	var req AuditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	for _, result := range req.Results {
		s.store.SaveAudit(store.AuditEntry{
			SessionID: req.SessionID,
			Event:     req.Event,
			RuleID:    result.RuleID,
			Passed:    result.Passed,
			Message:   result.Message,
			Severity:  string(result.Severity),
			Timestamp: req.Timestamp,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"config": s.policy,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","version":"1.0.0"}`))
}
