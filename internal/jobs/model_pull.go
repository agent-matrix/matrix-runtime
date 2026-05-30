package jobs

import (
	"context"
	"fmt"

	"github.com/agent-matrix/matrix-runtime/internal/cache"
	"github.com/agent-matrix/matrix-runtime/internal/hf"
)

// modelPullPayload is the payload for a model.pull job.
type modelPullPayload struct {
	Model    string `json:"model"`
	Revision string `json:"revision"`
}

// handleModelPull stages a model in the local cache. For the MVP it resolves
// the reference and writes metadata + the cache directory layout; full weight
// downloads are intentionally deferred.
func handleModelPull(ctx context.Context, m *Manager, j *Job) error {
	var p modelPullPayload
	if err := decodePayload(j.Payload, &p); err != nil {
		return err
	}
	if p.Model == "" {
		return fmt.Errorf("payload.model is required")
	}
	ref, err := hf.ParseRef(p.Model, p.Revision)
	if err != nil {
		return err
	}
	j.Emit("cache", EvStart, "Preparing model cache", nil)

	dir, err := m.Layout().WriteModelMetadata(cache.ModelMetadata{
		Model:     "hf:" + ref.RepoID(),
		Namespace: ref.Namespace,
		Name:      ref.Name,
		Revision:  ref.Revision,
		Source:    ref.Source,
		Extra:     map[string]any{"staged": true, "note": "weights not downloaded in MVP"},
	})
	if err != nil {
		return err
	}
	j.Emit("cache", EvOK, "Cache directory prepared", map[string]any{"dir": dir})
	j.setResult(map[string]any{
		"model":     "hf:" + ref.RepoID(),
		"revision":  ref.Revision,
		"cache_dir": dir,
		"staged":    true,
	})
	return nil
}
