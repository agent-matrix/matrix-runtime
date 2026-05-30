package jobs

import (
	"context"
	"fmt"
)

// handleModelPreload loads a model into a serving runtime (Ollama/vLLM/SGLang).
// Stubbed for the MVP.
func handleModelPreload(_ context.Context, _ *Manager, j *Job) error {
	j.Emit("preload", EvError, "model.preload is not implemented in this build", nil)
	return fmt.Errorf("model.preload is not implemented in the MVP")
}
