package store

import "time"

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Event     string    `json:"event"`
	RuleID    string    `json:"rule_id"`
	Passed    bool      `json:"passed"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

// Store is the interface for audit persistence.
type Store interface {
	// SaveAudit persists an audit entry.
	SaveAudit(entry AuditEntry) error

	// ListAudit returns recent audit entries.
	ListAudit(limit int) ([]AuditEntry, error)

	// Close releases resources.
	Close() error
}
