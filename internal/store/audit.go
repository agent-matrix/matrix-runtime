package store

import (
	"encoding/json"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/auth"
)

// AuditEvent is one row in the append-only audit log.
type AuditEvent struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Actor       string         `json:"actor"`
	Action      string         `json:"action"`
	Target      string         `json:"target"`
	IP          string         `json:"ip"`
	Status      string         `json:"status"`
	Meta        map[string]any `json:"meta"`
	CreatedAt   time.Time      `json:"created_at"`
}

// RecordAudit appends an audit event. Failures are non-fatal to callers (audit
// is best-effort) but the error is returned so it can be logged.
func (s *Store) RecordAudit(e AuditEvent) error {
	if e.Status == "" {
		e.Status = "success"
	}
	if e.ID == "" {
		e.ID = auth.NewID("au_")
	}
	metaB, _ := json.Marshal(e.Meta)
	_, err := s.exec(`INSERT INTO audit_events (id,workspace_id,actor,action,target,ip,status,meta_json,created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		e.ID, e.WorkspaceID, e.Actor, e.Action, e.Target, e.IP, e.Status, string(metaB), nowRFC())
	return err
}

// ListAudit returns a workspace's most recent audit events (newest first).
func (s *Store) ListAudit(workspaceID string, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.query(`SELECT id,workspace_id,actor,action,target,ip,status,meta_json,created_at
		FROM audit_events WHERE workspace_id=? ORDER BY created_at DESC LIMIT ?`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		var meta, created string
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.Actor, &e.Action, &e.Target, &e.IP, &e.Status, &meta, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(meta), &e.Meta)
		e.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, e)
	}
	return out, rows.Err()
}
