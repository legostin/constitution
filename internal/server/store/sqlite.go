package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite for audit persistence.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite database at the given path.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open failed: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: WAL mode failed: %w", err)
	}

	// Create audit table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			event TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			passed BOOLEAN NOT NULL,
			message TEXT,
			severity TEXT,
			timestamp DATETIME NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: create table failed: %w", err)
	}

	// Create index for session queries
	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_log(session_id, timestamp)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: create index failed: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) SaveAudit(entry AuditEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_log (session_id, event, rule_id, passed, message, severity, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.SessionID, entry.Event, entry.RuleID, entry.Passed, entry.Message, entry.Severity, entry.Timestamp,
	)
	return err
}

func (s *SQLiteStore) ListAudit(limit int) ([]AuditEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, event, rule_id, passed, message, severity, timestamp FROM audit_log ORDER BY timestamp DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var ts string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Event, &e.RuleID, &e.Passed, &e.Message, &e.Severity, &ts); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
