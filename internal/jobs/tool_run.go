package jobs

import (
	"context"
	"fmt"
)

// handleToolRun runs a tool worker. Stubbed for the MVP.
func handleToolRun(_ context.Context, _ *Manager, j *Job) error {
	j.Emit("tool_run", EvError, "tool.run is not implemented in this build", nil)
	return fmt.Errorf("tool.run is not implemented in the MVP")
}
