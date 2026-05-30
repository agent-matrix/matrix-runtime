package jobs

import (
	"context"
	"fmt"
)

// handleMCPRun runs an MCP server persistently. The type is defined now; the
// MVP behaviour is stubbed (use mcp.test for short-lived sandboxes).
func handleMCPRun(_ context.Context, _ *Manager, j *Job) error {
	j.Emit("mcp_run", EvError, "mcp.run is not implemented in this build", nil)
	return fmt.Errorf("mcp.run is not implemented in the MVP; use mcp.test")
}
