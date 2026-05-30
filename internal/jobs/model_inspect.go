package jobs

import (
	"context"
	"fmt"

	"github.com/agent-matrix/matrix-runtime/internal/models"
)

// modelInspectPayload is the payload for a model.inspect job.
type modelInspectPayload struct {
	Model    string `json:"model"`
	Revision string `json:"revision"`
}

// handleModelInspect resolves a Hugging Face model and extracts metadata.
func handleModelInspect(ctx context.Context, m *Manager, j *Job) error {
	var p modelInspectPayload
	if err := decodePayload(j.Payload, &p); err != nil {
		return err
	}
	if p.Model == "" {
		return fmt.Errorf("payload.model is required (e.g. hf:Qwen/Qwen2.5-7B-Instruct)")
	}
	j.Emit("resolve", EvStart, "Resolving model "+p.Model, nil)

	meta, err := models.Inspect(ctx, p.Model, p.Revision, m.Config().HFToken)
	if err != nil {
		return err
	}
	j.Emit("inspect", EvOK, "Model metadata resolved", map[string]any{
		"recommended_runtime": meta.RecommendedRuntime,
	})
	j.setResult(meta)
	return nil
}
