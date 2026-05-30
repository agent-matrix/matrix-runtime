package api

import (
	"encoding/json"
	"net/http"

	"github.com/agent-matrix/matrix-runtime/internal/jobs"
)

func (s *Server) handleCreateSandbox(w http.ResponseWriter, r *http.Request) {
	var req jobs.SandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	sessionID, j, err := s.manager.CreateSandbox(req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"session_id": sessionID,
		"job_id":     j.ID,
		"status":     "starting",
		"expires_at": j.Snapshot().ExpiresAt,
		"events_url": "/v1/sandbox/sessions/" + sessionID + "/events",
	})
}

func (s *Server) handleGetSandbox(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	j, ok := s.manager.SandboxJob(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "sandbox session not found")
		return
	}
	snap := j.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"job_id":     j.ID,
		"status":     snap.Status,
		"expires_at": snap.ExpiresAt,
		"result":     snap.Result,
	})
}

func (s *Server) handleSandboxEvents(w http.ResponseWriter, r *http.Request) {
	j, ok := s.manager.SandboxJob(r.PathValue("session_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "sandbox session not found")
		return
	}
	streamEvents(w, r, j.Bus())
}

func (s *Server) handleSandboxTools(w http.ResponseWriter, r *http.Request) {
	tools, err := s.manager.SandboxTools(r.PathValue("session_id"))
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (s *Server) handleSandboxToolCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "tool name is required")
		return
	}
	result, err := s.manager.CallSandboxTool(r.Context(), r.PathValue("session_id"), req.Name, req.Arguments)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": json.RawMessage(result)})
}

func (s *Server) handleDeleteSandbox(w http.ResponseWriter, r *http.Request) {
	j, ok := s.manager.SandboxJob(r.PathValue("session_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "sandbox session not found")
		return
	}
	status, _ := s.manager.Cancel(j.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}
