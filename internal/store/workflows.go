package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/auth"
)

type WorkflowRun struct {
	ID          string `json:"run_id"`
	WorkspaceID string `json:"workspace_id"`
	TraceID     string `json:"trace_id"`
	PlanID      string `json:"plan_id"`
	GraphID     string `json:"graph_id"`
	Status      string `json:"status"`
	Checkpoint  string `json:"checkpoint,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type WorkflowEvent struct {
	ID         string         `json:"event_id"`
	RunID      string         `json:"run_id"`
	Seq        int            `json:"seq"`
	EventType  string         `json:"event_type"`
	FromStatus string         `json:"from_status,omitempty"`
	ToStatus   string         `json:"to_status,omitempty"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  string         `json:"created_at"`
}

func (s *Store) CreateWorkflow(workspaceID, traceID, planID, graphID string) (*WorkflowRun, error) {
	if traceID == "" {
		return nil, errors.New("trace_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r := &WorkflowRun{
		ID: auth.NewID("run_"), WorkspaceID: workspaceID, TraceID: traceID,
		PlanID: planID, GraphID: graphID, Status: "queued",
		CreatedAt: now, UpdatedAt: now,
	}
	_, err := s.exec(`INSERT INTO workflow_runs
		(id,workspace_id,trace_id,plan_id,graph_id,status,checkpoint,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		r.ID, r.WorkspaceID, r.TraceID, r.PlanID, r.GraphID, r.Status, "", now, now)
	if err != nil {
		return nil, err
	}
	if _, err := s.AppendWorkflowEvent(r.ID, "created", "", "queued", map[string]any{"trace_id": traceID}); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) Workflow(runID string) (*WorkflowRun, error) {
	var r WorkflowRun
	row := s.queryRow(`SELECT id,workspace_id,trace_id,plan_id,graph_id,status,checkpoint,created_at,updated_at
		FROM workflow_runs WHERE id=?`, runID)
	if err := row.Scan(&r.ID, &r.WorkspaceID, &r.TraceID, &r.PlanID, &r.GraphID,
		&r.Status, &r.Checkpoint, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) AppendWorkflowEvent(runID, eventType, fromStatus, toStatus string, payload map[string]any) (*WorkflowEvent, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	var seq int
	if err := s.queryRow(`SELECT COALESCE(MAX(seq),0)+1 FROM workflow_events WHERE run_id=?`, runID).Scan(&seq); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(payload)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	ev := &WorkflowEvent{
		ID: auth.NewID("wev_"), RunID: runID, Seq: seq, EventType: eventType,
		FromStatus: fromStatus, ToStatus: toStatus, Payload: payload, CreatedAt: now,
	}
	if _, err := s.exec(`INSERT INTO workflow_events
		(id,run_id,seq,event_type,from_status,to_status,payload_json,created_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		ev.ID, runID, seq, eventType, fromStatus, toStatus, string(raw), now); err != nil {
		return nil, err
	}
	return ev, nil
}

func (s *Store) TransitionWorkflow(runID, target, checkpoint, eventType string, payload map[string]any) (*WorkflowRun, *WorkflowEvent, error) {
	current, err := s.Workflow(runID)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if eventType == "" {
		eventType = "transition"
	}
	if _, err := s.exec(`UPDATE workflow_runs SET status=?,checkpoint=?,updated_at=? WHERE id=?`,
		target, checkpoint, now, runID); err != nil {
		return nil, nil, err
	}
	ev, err := s.AppendWorkflowEvent(runID, eventType, current.Status, target, payload)
	if err != nil {
		return nil, nil, err
	}
	current.Status, current.Checkpoint, current.UpdatedAt = target, checkpoint, now
	return current, ev, nil
}

func (s *Store) WorkflowEvents(runID string) ([]WorkflowEvent, error) {
	rows, err := s.query(`SELECT id,run_id,seq,event_type,from_status,to_status,payload_json,created_at
		FROM workflow_events WHERE run_id=? ORDER BY seq ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkflowEvent{}
	for rows.Next() {
		var ev WorkflowEvent
		var raw string
		if err := rows.Scan(&ev.ID, &ev.RunID, &ev.Seq, &ev.EventType, &ev.FromStatus,
			&ev.ToStatus, &raw, &ev.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &ev.Payload); err != nil {
			return nil, fmt.Errorf("decode workflow event %s: %w", ev.ID, err)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
