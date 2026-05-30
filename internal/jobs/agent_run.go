package jobs

import (
	"context"
	"fmt"
)

// handleAgentRun runs an agent backed by a MatrixLLM model and MCP tools.
// Stubbed for the MVP.
func handleAgentRun(_ context.Context, _ *Manager, j *Job) error {
	j.Emit("agent_run", EvError, "agent.run is not implemented in this build", nil)
	return fmt.Errorf("agent.run is not implemented in the MVP")
}
