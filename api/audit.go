package api

import (
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/agent-matrix/matrix-runtime/internal/store"
)

// clientIP best-effort extracts the caller IP, honoring common proxy headers
// (the API typically runs behind Cloudflare / an ingress).
func clientIP(r *http.Request) string {
	if v := r.Header.Get("CF-Connecting-IP"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return strings.TrimSpace(strings.Split(v, ",")[0])
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// audit records a sensitive action. It is best-effort: a nil store or a write
// error never affects the request outcome (only logged).
func (s *Server) audit(r *http.Request, workspaceID, actor, action, target, status string, meta map[string]any) {
	if s.store == nil {
		return
	}
	if err := s.store.RecordAudit(store.AuditEvent{
		WorkspaceID: workspaceID,
		Actor:       actor,
		Action:      action,
		Target:      target,
		IP:          clientIP(r),
		Status:      status,
		Meta:        meta,
	}); err != nil {
		log.Printf("audit: could not record %s: %v", action, err)
	}
}

// truncate shortens s to at most n characters for audit targets.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// handleCloudAudit returns the workspace's recent audit events.
func (s *Server) handleCloudAudit(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	events, err := s.store.ListAudit(u.WorkspaceID, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load audit log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
