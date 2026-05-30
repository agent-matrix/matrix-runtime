package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/agent-matrix/matrix-runtime/internal/hf"
	"github.com/agent-matrix/matrix-runtime/internal/jobs"
	"github.com/agent-matrix/matrix-runtime/internal/models"
	"github.com/agent-matrix/matrix-runtime/internal/store"
)

// currentUser resolves the session bearer token to a user, or returns false.
func (s *Server) currentUser(r *http.Request) (*store.User, bool) {
	if s.store == nil {
		return nil, false
	}
	u, err := s.store.UserBySession(bearer(r))
	if err != nil {
		return nil, false
	}
	return u, true
}

// handleHFSearch proxies the Hugging Face model search server-side (avoiding
// browser CORS) for the console's generic Import Model flow. On any failure it
// returns 200 with live=false and an empty list so the UI can fall back to
// sample data gracefully.
func (s *Server) handleHFSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	task := r.URL.Query().Get("task")
	limit := 16
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	items, err := hf.NewClient(s.cfg.HFToken).Search(r.Context(), q, task, limit)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "live": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "live": true})
}

// resolveReq describes a generic model source to resolve into a profile preview.
type resolveReq struct {
	SourceType string `json:"sourceType"`
	SourceURI  string `json:"sourceUri"`
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"` // e.g. HF model id
	Model      string `json:"model"`      // e.g. hf:owner/name
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	Private    bool   `json:"private"`
}

// handleResolveSource resolves a source into a model-profile preview. Hugging
// Face is resolved for real via model.inspect; other sources are constructed
// from the supplied location.
func (s *Server) handleResolveSource(w http.ResponseWriter, r *http.Request) {
	var req resolveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	isHF := req.SourceType == "huggingface" || strings.HasPrefix(req.Model, "hf:") || (req.Provider == "Hugging Face")
	if isHF {
		id := req.ExternalID
		if id == "" {
			id = strings.TrimPrefix(req.Model, "hf:")
		}
		meta, err := models.Inspect(r.Context(), "hf:"+id, "main", s.cfg.HFToken)
		if err != nil {
			writeError(w, http.StatusBadGateway, "could not resolve model: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"source_type": "huggingface", "provider": "Hugging Face", "external_id": id,
			"display_name": id, "source_uri": "hf:" + id,
			"task": meta.PipelineTag, "library": meta.LibraryName, "license": meta.License,
			"requires_gpu": meta.RequiresGPU, "recommended_runtime": meta.RecommendedRuntime,
			"estimated_parameters": meta.EstimatedParameters, "tags": meta.Tags, "private": req.Private,
		})
		return
	}
	// Generic (GitHub/GitLab/S3/R2/Ollama/URL): construct a profile from the form.
	uri := req.SourceURI
	if req.Path != "" {
		uri = strings.TrimRight(uri, "/") + "/" + strings.TrimLeft(req.Path, "/")
	}
	name := req.ExternalID
	if name == "" {
		name = uri
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source_type": req.SourceType, "provider": req.Provider, "external_id": name,
		"display_name": name, "source_uri": uri, "task": "text-generation",
		"library": "custom", "license": "review required", "private": req.Private,
		"recommended_runtime": "vLLM / SGLang",
	})
}

// profileJSON shapes a stored profile for the API.
func profileJSON(p store.ModelProfile) map[string]any {
	return map[string]any{
		"id": p.ID, "source_type": p.SourceType, "source_uri": p.SourceURI,
		"provider": p.Provider, "external_id": p.ExternalID, "display_name": p.DisplayName,
		"task": p.Task, "library": p.Library, "license": p.License, "tags": p.Tags,
		"status": p.Status, "created_at": p.CreatedAt, "metadata": p.Metadata,
	}
}

// handleListProfiles lists model profiles for the caller's workspace.
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	list, err := s.store.ListProfiles(u.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, p := range list {
		out = append(out, profileJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": out})
}

type importReq struct {
	SourceType  string         `json:"source_type"`
	SourceURI   string         `json:"source_uri"`
	Provider    string         `json:"provider"`
	ExternalID  string         `json:"external_id"`
	DisplayName string         `json:"display_name"`
	Task        string         `json:"task"`
	Library     string         `json:"library"`
	License     string         `json:"license"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
}

// handleImportProfile creates a model profile (status profile_only).
func (s *Server) handleImportProfile(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var req importReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.ExternalID
	}
	p, err := s.store.CreateProfile(store.ModelProfile{
		WorkspaceID: u.WorkspaceID, SourceType: req.SourceType, SourceURI: req.SourceURI,
		Provider: req.Provider, ExternalID: req.ExternalID, DisplayName: req.DisplayName,
		Task: req.Task, Library: req.Library, License: req.License, Tags: req.Tags,
		Metadata: req.Metadata, Status: "profile_only",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, u.WorkspaceID, u.ID, "model.imported", p.DisplayName, "success", map[string]any{"provider": p.Provider, "external_id": p.ExternalID})
	writeJSON(w, http.StatusCreated, map[string]any{"profile": profileJSON(*p)})
}

type attachReq struct {
	RuntimeID     string `json:"runtimeId"`
	InstallMode   string `json:"installMode"`
	ServingEngine string `json:"servingEngine"`
}

// handleAttachProfile creates an installation row and a model.attach job that
// streams real progress and persists it.
func (s *Server) handleAttachProfile(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	pid := r.PathValue("id")
	p, err := s.store.GetProfile(u.WorkspaceID, pid)
	if err != nil {
		writeError(w, http.StatusNotFound, "model profile not found")
		return
	}
	var req attachReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.RuntimeID == "" {
		writeError(w, http.StatusBadRequest, "runtimeId is required")
		return
	}
	if req.InstallMode == "" {
		req.InstallMode = "pull_from_source"
	}
	inst, err := s.store.CreateInstallation(store.ModelInstallation{
		WorkspaceID: u.WorkspaceID, ModelProfileID: p.ID, RuntimeID: req.RuntimeID,
		InstallMode: req.InstallMode, ServingEngine: req.ServingEngine, Status: "queued",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.SetProfileStatus(p.ID, "queued")

	model := p.SourceURI
	if p.Provider == "Hugging Face" {
		model = "hf:" + p.ExternalID
	}
	payload, _ := json.Marshal(map[string]any{
		"installation_id": inst.ID, "profile_id": p.ID, "model": model, "provider": p.Provider,
		"runtime_id": req.RuntimeID, "install_mode": req.InstallMode, "serving_engine": req.ServingEngine,
	})
	job, err := s.manager.Create(jobs.CreateRequest{Type: jobs.TypeModelAttach, TTLSeconds: 180, Payload: payload})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	_ = s.store.SetInstallationJob(inst.ID, job.ID)
	s.audit(r, u.WorkspaceID, u.ID, "model.attached", p.DisplayName, "success", map[string]any{"runtime_id": req.RuntimeID, "job_id": job.ID})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"installation_id": inst.ID,
		"profile_id":      p.ID,
		"job_id":          job.ID,
		"events_url":      "/v1/jobs/" + job.ID + "/events",
	})
}

// installationJSON shapes a stored installation for the API.
func installationJSON(in store.ModelInstallation) map[string]any {
	return map[string]any{
		"id": in.ID, "model_profile_id": in.ModelProfileID, "runtime_id": in.RuntimeID,
		"install_mode": in.InstallMode, "serving_engine": in.ServingEngine,
		"status": in.Status, "progress": in.Progress, "local_path": in.LocalPath,
		"endpoint_url": in.EndpointURL, "job_id": in.JobID,
		"model_name": in.ModelName, "provider": in.Provider, "updated_at": in.UpdatedAt,
	}
}

// handleListInstallations lists runtime-cache installations for the workspace.
func (s *Server) handleListInstallations(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	list, err := s.store.ListInstallations(u.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, in := range list {
		out = append(out, installationJSON(in))
	}
	writeJSON(w, http.StatusOK, map[string]any{"installations": out})
}
