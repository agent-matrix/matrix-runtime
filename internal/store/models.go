package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/auth"
)

// ModelProfile is lightweight model metadata MatrixCloud knows about — a
// "profile only" record that may later be downloaded/attached/made ready.
type ModelProfile struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
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
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ModelInstallation is a physical install/attachment of a profile to a runtime.
type ModelInstallation struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	ModelProfileID string    `json:"model_profile_id"`
	RuntimeID      string    `json:"runtime_id"`
	InstallMode    string    `json:"install_mode"`
	ServingEngine  string    `json:"serving_engine"`
	Status         string    `json:"status"`
	Progress       int       `json:"progress"`
	LocalPath      string    `json:"local_path"`
	EndpointURL    string    `json:"endpoint_url"`
	JobID          string    `json:"job_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Joined display fields (from the profile) for the Runtime Cache view.
	ModelName string `json:"model_name,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339) }

// CreateProfile inserts (or updates by external id within the workspace) a model
// profile and returns it.
func (s *Store) CreateProfile(p ModelProfile) (*ModelProfile, error) {
	if p.WorkspaceID == "" {
		return nil, errors.New("workspace_id required")
	}
	p.ID = auth.NewID("mp_")
	if p.Status == "" {
		p.Status = "profile_only"
	}
	now := nowRFC()
	tags, _ := json.Marshal(p.Tags)
	meta, _ := json.Marshal(p.Metadata)
	_, err := s.exec(`
		INSERT INTO model_profiles
		  (id,workspace_id,source_type,source_uri,provider,external_id,display_name,task,library,license,tags_json,metadata_json,status,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.WorkspaceID, p.SourceType, p.SourceURI, p.Provider, p.ExternalID, p.DisplayName, p.Task, p.Library, p.License, string(tags), string(meta), p.Status, now, now)
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, now)
	p.UpdatedAt = p.CreatedAt
	return &p, nil
}

func scanProfile(rows interface{ Scan(...any) error }) (*ModelProfile, error) {
	var p ModelProfile
	var tags, meta, created, updated string
	if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.SourceType, &p.SourceURI, &p.Provider, &p.ExternalID, &p.DisplayName, &p.Task, &p.Library, &p.License, &tags, &meta, &p.Status, &created, &updated); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tags), &p.Tags)
	_ = json.Unmarshal([]byte(meta), &p.Metadata)
	p.CreatedAt, _ = time.Parse(time.RFC3339, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return &p, nil
}

const profileCols = `id,workspace_id,source_type,source_uri,provider,external_id,display_name,task,library,license,tags_json,metadata_json,status,created_at,updated_at`

// GetProfile returns a profile by id (workspace-scoped).
func (s *Store) GetProfile(workspaceID, id string) (*ModelProfile, error) {
	row := s.queryRow(`SELECT `+profileCols+` FROM model_profiles WHERE id=? AND workspace_id=?`, id, workspaceID)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ListProfiles returns all profiles for a workspace, newest first.
func (s *Store) ListProfiles(workspaceID string) ([]ModelProfile, error) {
	rows, err := s.query(`SELECT `+profileCols+` FROM model_profiles WHERE workspace_id=? ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ModelProfile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// SetProfileStatus updates a profile's status.
func (s *Store) SetProfileStatus(id, status string) error {
	_, err := s.exec(`UPDATE model_profiles SET status=?, updated_at=? WHERE id=?`, status, nowRFC(), id)
	return err
}

// CreateInstallation inserts a runtime installation row.
func (s *Store) CreateInstallation(in ModelInstallation) (*ModelInstallation, error) {
	in.ID = auth.NewID("mi_")
	if in.Status == "" {
		in.Status = "queued"
	}
	now := nowRFC()
	_, err := s.exec(`
		INSERT INTO model_runtime_installations
		  (id,workspace_id,model_profile_id,runtime_id,install_mode,serving_engine,status,progress,local_path,endpoint_url,job_id,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.ID, in.WorkspaceID, in.ModelProfileID, in.RuntimeID, in.InstallMode, in.ServingEngine, in.Status, in.Progress, in.LocalPath, in.EndpointURL, in.JobID, now, now)
	if err != nil {
		return nil, err
	}
	in.CreatedAt, _ = time.Parse(time.RFC3339, now)
	in.UpdatedAt = in.CreatedAt
	return &in, nil
}

// UpdateInstallation sets status/progress (and optional paths) on an installation.
func (s *Store) UpdateInstallation(id, status string, progress int, localPath, endpointURL string) error {
	_, err := s.exec(`
		UPDATE model_runtime_installations
		SET status=?, progress=?,
		    local_path=CASE WHEN ?<>'' THEN ? ELSE local_path END,
		    endpoint_url=CASE WHEN ?<>'' THEN ? ELSE endpoint_url END,
		    updated_at=?
		WHERE id=?`,
		status, progress, localPath, localPath, endpointURL, endpointURL, nowRFC(), id)
	return err
}

// SetInstallationJob records the job id driving an installation.
func (s *Store) SetInstallationJob(id, jobID string) error {
	_, err := s.exec(`UPDATE model_runtime_installations SET job_id=?, updated_at=? WHERE id=?`, jobID, nowRFC(), id)
	return err
}

// ListInstallations returns installations for a workspace joined with profile
// display fields (for the Runtime Cache view), newest first.
func (s *Store) ListInstallations(workspaceID string) ([]ModelInstallation, error) {
	rows, err := s.query(`
		SELECT i.id,i.workspace_id,i.model_profile_id,i.runtime_id,i.install_mode,i.serving_engine,i.status,i.progress,i.local_path,i.endpoint_url,i.job_id,i.created_at,i.updated_at,
		       COALESCE(p.display_name,''), COALESCE(p.provider,'')
		FROM model_runtime_installations i
		LEFT JOIN model_profiles p ON p.id = i.model_profile_id
		WHERE i.workspace_id=? ORDER BY i.created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ModelInstallation{}
	for rows.Next() {
		var in ModelInstallation
		var created, updated string
		if err := rows.Scan(&in.ID, &in.WorkspaceID, &in.ModelProfileID, &in.RuntimeID, &in.InstallMode, &in.ServingEngine, &in.Status, &in.Progress, &in.LocalPath, &in.EndpointURL, &in.JobID, &created, &updated, &in.ModelName, &in.Provider); err != nil {
			return nil, err
		}
		in.CreatedAt, _ = time.Parse(time.RFC3339, created)
		in.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, in)
	}
	return out, rows.Err()
}
