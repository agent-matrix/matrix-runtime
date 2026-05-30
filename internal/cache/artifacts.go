package cache

import (
	"os"
	"path/filepath"
)

// ArtifactDir returns (creating if needed) a per-job artifact directory.
func (l *Layout) ArtifactDir(jobID string) (string, error) {
	dir := filepath.Join(l.Jobs(), jobID, "artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// MCPScratchDir returns (creating if needed) a temporary working directory for
// an MCP sandbox keyed by job ID.
func (l *Layout) MCPScratchDir(jobID string) (string, error) {
	dir := filepath.Join(l.MCP(), jobID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// RemoveJob deletes all scratch and artifact data for a job.
func (l *Layout) RemoveJob(jobID string) {
	_ = os.RemoveAll(filepath.Join(l.MCP(), jobID))
	_ = os.RemoveAll(filepath.Join(l.Jobs(), jobID))
}
