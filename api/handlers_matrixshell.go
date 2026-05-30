package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/jobs"
	"github.com/agent-matrix/matrix-runtime/internal/matrixshell"
)

// requireMatrixShell rejects requests when MatrixShell is disabled. It executes
// commands in a local sandbox, so it is off by default in production modes.
func (s *Server) requireMatrixShell(w http.ResponseWriter) bool {
	if !s.cfg.MatrixShellEnabled {
		writeError(w, http.StatusForbidden, "MatrixShell is disabled (set MATRIX_SHELL_ENABLED=true to enable)")
		return false
	}
	return true
}

// handleMatrixShellStatus reports whether MatrixShell is installed in the local
// Python sandbox, with its version and paths.
func (s *Server) handleMatrixShellStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireMatrixShell(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, matrixshell.GetStatus(ctx, s.cfg.DataDir))
}

// handleMatrixShellInstall starts a job that creates the sandbox venv and
// installs MatrixShell from git, streaming real output over SSE.
func (s *Server) handleMatrixShellInstall(w http.ResponseWriter, _ *http.Request) {
	if !s.requireMatrixShell(w) {
		return
	}
	job, err := s.manager.Create(jobs.CreateRequest{Type: jobs.TypeMatrixShellInstall, TTLSeconds: 600})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":     job.ID,
		"events_url": "/v1/jobs/" + job.ID + "/events",
	})
}

// handleMatrixShellExec runs a command inside the local MatrixShell sandbox
// (real execution with the venv on PATH) and returns stdout/stderr/exit.
func (s *Server) handleMatrixShellExec(w http.ResponseWriter, r *http.Request) {
	if !s.requireMatrixShell(w) {
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	wsID, actor := "", "operator"
	if u, ok := s.currentUser(r); ok {
		wsID, actor = u.WorkspaceID, u.ID
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	res, err := matrixshell.Exec(ctx, s.cfg.DataDir, req.Command)
	if err != nil {
		if errors.Is(err, matrixshell.ErrBlocked) {
			s.audit(r, wsID, actor, "matrixshell.exec", truncate(req.Command, 200), "failure", map[string]any{"reason": "denylist"})
			writeError(w, http.StatusForbidden, "refused by safety denylist")
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	s.audit(r, wsID, actor, "matrixshell.exec", truncate(req.Command, 200), "success", map[string]any{"exit_code": res.ExitCode})
	writeJSON(w, http.StatusOK, map[string]any{
		"command": req.Command, "stdout": res.Stdout, "stderr": res.Stderr, "exit_code": res.ExitCode,
	})
}
