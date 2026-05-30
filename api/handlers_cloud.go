package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/store"
)

// staleAfter marks a runtime offline if it hasn't sent a heartbeat recently.
const runtimeStaleAfter = 90 * time.Second

// handleCloudListRuntimes returns the calling workspace's registered runtimes.
func (s *Server) handleCloudListRuntimes(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	list, err := s.store.ListRuntimes(u.WorkspaceID, runtimeStaleAfter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list runtimes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runtimes": list})
}

// handleCloudRegisterRuntime is called by a remote sandbox (e.g. a duplicated
// HF Space) presenting a workspace join token. It registers the runtime and
// returns a long-lived runtime token used for subsequent heartbeats.
func (s *Server) handleCloudRegisterRuntime(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct {
		JoinToken string   `json:"join_token"`
		Name      string   `json:"name"`
		Kind      string   `json:"kind"`
		Mode      string   `json:"mode"`
		URL       string   `json:"url"`
		HFSpace   string   `json:"hf_space"`
		Region    string   `json:"region"`
		Version   string   `json:"version"`
		Caps      []string `json:"caps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	workspaceID, err := s.store.RedeemJoinToken(req.JoinToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if req.Name == "" {
		req.Name = req.HFSpace
	}
	if req.Kind == "" {
		req.Kind = "hf-space"
	}
	rt, token, err := s.store.RegisterRuntime(store.Runtime{
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Mode:        req.Mode,
		Kind:        req.Kind,
		URL:         req.URL,
		HFSpace:     req.HFSpace,
		Region:      req.Region,
		Version:     req.Version,
		Caps:        req.Caps,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not register runtime")
		return
	}
	s.audit(r, workspaceID, rt.ID, "runtime.registered", rt.Name, "success", map[string]any{"kind": rt.Kind, "hf_space": rt.HFSpace})
	writeJSON(w, http.StatusCreated, map[string]any{"runtime": rt, "runtime_token": token})
}

// handleCloudHeartbeat updates a runtime's status using its runtime token
// (Authorization: Bearer <runtime_token>).
func (s *Server) handleCloudHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct {
		Status string   `json:"status"`
		Caps   []string `json:"caps"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	rt, err := s.store.HeartbeatRuntime(bearer(r), req.Status, req.Caps)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid runtime token")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not record heartbeat")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runtime": rt})
}

// handleCloudListJoinTokens lists the workspace's active join tokens.
func (s *Server) handleCloudListJoinTokens(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	list, err := s.store.ListJoinTokens(u.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list join tokens")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"join_tokens": list})
}

// handleCloudMintJoinToken mints a new join token for the workspace. The secret
// is returned exactly once.
func (s *Server) handleCloudMintJoinToken(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var req struct {
		Label      string `json:"label"`
		MaxUses    int    `json:"max_uses"`
		TTLMinutes int    `json:"ttl_minutes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	ttl := time.Duration(req.TTLMinutes) * time.Minute
	if req.TTLMinutes == 0 {
		ttl = 24 * time.Hour
	}
	jt, secret, err := s.store.MintJoinToken(u.WorkspaceID, u.ID, req.Label, req.MaxUses, ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not mint join token")
		return
	}
	s.audit(r, u.WorkspaceID, u.ID, "runtime.join_token.created", jt.ID, "success", map[string]any{"label": jt.Label, "max_uses": jt.MaxUses})
	writeJSON(w, http.StatusCreated, map[string]any{"join_token": jt, "secret": secret})
}

// handleCloudListProviders lists BYO provider credentials (hints only).
func (s *Server) handleCloudListProviders(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	list, err := s.store.ListProviderCredentials(u.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list providers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": list})
}

// handleCloudSetProvider stores (encrypted) a BYO provider token — e.g. a user
// plugging in their own Hugging Face account to use HF LLMs inside MatrixCloud.
func (s *Server) handleCloudSetProvider(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var req struct {
		Provider string         `json:"provider"`
		Label    string         `json:"label"`
		Secret   string         `json:"secret"`
		Meta     map[string]any `json:"meta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Provider == "" || req.Secret == "" {
		writeError(w, http.StatusBadRequest, "provider and secret are required")
		return
	}
	pc, err := s.store.SetProviderCredential(u.WorkspaceID, u.ID, req.Provider, req.Label, req.Secret, req.Meta)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save credential")
		return
	}
	s.audit(r, u.WorkspaceID, u.ID, "provider.credential.added", req.Provider, "success", map[string]any{"label": pc.Label, "hint": pc.Hint})
	writeJSON(w, http.StatusCreated, map[string]any{"provider": pc})
}

// handleCloudUsage returns the workspace's usage in the trailing 30 days.
func (s *Server) handleCloudUsage(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	used, err := s.store.UsageSince(u.WorkspaceID, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load usage")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"since": since.UTC().Format(time.RFC3339), "usage": used})
}
