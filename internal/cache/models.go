package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ModelMetadata is the metadata.json persisted alongside a cached model.
type ModelMetadata struct {
	Model     string         `json:"model"`
	Namespace string         `json:"namespace"`
	Name      string         `json:"name"`
	Revision  string         `json:"revision"`
	Source    string         `json:"source"`
	CachedAt  time.Time      `json:"cached_at"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// WriteModelMetadata creates the model cache directory and writes metadata.json.
func (l *Layout) WriteModelMetadata(meta ModelMetadata) (string, error) {
	dir := l.ModelDir(meta.Namespace, meta.Name)
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0o755); err != nil {
		return "", err
	}
	if meta.CachedAt.IsZero() {
		meta.CachedAt = time.Now().UTC()
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

// ReadModelMetadata loads metadata.json for a cached model, if present.
func (l *Layout) ReadModelMetadata(namespace, name string) (*ModelMetadata, error) {
	path := filepath.Join(l.ModelDir(namespace, name), "metadata.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m ModelMetadata
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
