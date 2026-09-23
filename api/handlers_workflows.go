package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agent-matrix/matrix-runtime/internal/store"
	"github.com/agent-matrix/matrix-runtime/internal/workflow"
)

type workflowCreateRequest struct {
	TraceID string `json:"trace_id"`
	PlanID  string `json:"plan_id"`
	GraphID string `json:"graph_id"`
}

type workflowTransitionRequest struct {
	Target     string         `json:"target"`
	Checkpoint string         `json:"checkpoint,omitempty"`
	EventType  string         `json:"event_type,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

func (s *Server) requestWorkspace(r *http.Request) string {
	if s.store != nil {
		if tok := bearer(r); tok != "" {
			if u, err := s.store.UserBySession(tok); err == nil {
				return u.WorkspaceID
			}
		}
	}
	return "local"
}

func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "durable store unavailable")
		return
	}
	var req workflowCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	run, err := s.store.CreateWorkflow(s.requestWorkspace(r), req.TraceID, req.PlanID, req.GraphID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) ownedWorkflow(r *http.Request) (*store.WorkflowRun, bool) {
	run, err := s.store.Workflow(r.PathValue("run_id"))
	if err != nil {
		return nil, false
	}
	return run, run.WorkspaceID == s.requestWorkspace(r) || s.requestWorkspace(r) == "local"
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	run, ok := s.ownedWorkflow(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleWorkflowEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ownedWorkflow(r); !ok {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	events, err := s.store.WorkflowEvents(r.PathValue("run_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) transitionWorkflow(w http.ResponseWriter, r *http.Request, forcedTarget, forcedType string) {
	run, ok := s.ownedWorkflow(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	req := workflowTransitionRequest{Target: forcedTarget, EventType: forcedType}
	if forcedTarget == "" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	}
	if err := workflow.ValidateTransition(run.Status, req.Target); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	updated, event, err := s.store.TransitionWorkflow(run.ID, req.Target, req.Checkpoint, req.EventType, req.Payload)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "workflow not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": updated, "event": event})
}

func (s *Server) handleWorkflowTransition(w http.ResponseWriter, r *http.Request) {
	s.transitionWorkflow(w, r, "", "")
}
func (s *Server) handleWorkflowResume(w http.ResponseWriter, r *http.Request) {
	s.transitionWorkflow(w, r, "running", "resumed")
}
func (s *Server) handleWorkflowCancel(w http.ResponseWriter, r *http.Request) {
	s.transitionWorkflow(w, r, "cancelled", "cancelled")
}
func (s *Server) handleWorkflowRollback(w http.ResponseWriter, r *http.Request) {
	s.transitionWorkflow(w, r, "rolling_back", "rollback_requested")
}
