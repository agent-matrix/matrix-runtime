package jobs

import (
	"context"
	"fmt"
	"os"

	"github.com/agent-matrix/matrix-runtime/internal/logs"
	"github.com/agent-matrix/matrix-runtime/internal/mcp"
	"github.com/agent-matrix/matrix-runtime/internal/security"
)

// mcpTestPayload is the payload for an mcp.test (and sandbox) job.
type mcpTestPayload struct {
	Runtime      string            `json:"runtime"`
	Transport    string            `json:"transport"`
	StartCommand string            `json:"start_command"`
	Env          map[string]string `json:"env,omitempty"`
}

// handleMCPTest validates and runs an MCP server in a temporary sandbox for the
// job's TTL. It initialises the server, lists tools, then keeps the process
// alive (allowing tools/call) until the TTL expires or the job is cancelled.
func handleMCPTest(ctx context.Context, m *Manager, j *Job) error {
	var p mcpTestPayload
	if err := decodePayload(j.Payload, &p); err != nil {
		return err
	}
	if p.Transport != "" && p.Transport != "stdio" {
		return fmt.Errorf("unsupported transport %q (only stdio is supported)", p.Transport)
	}

	// 1. Validate the start command.
	tokens, err := security.ValidateCommand(p.StartCommand)
	if err != nil {
		return err
	}
	if err := security.CheckNoRawSecrets(p.Env); err != nil {
		return err
	}
	j.Emit("validate", EvOK, "Command accepted", map[string]any{"program": tokens[0]})

	// 2. Create a temporary working directory.
	workDir, err := m.Layout().MCPScratchDir(j.ID)
	if err != nil {
		return fmt.Errorf("create sandbox dir: %w", err)
	}
	j.Emit("sandbox", EvStart, "Creating temporary sandbox", map[string]any{"work_dir": workDir})

	// 3. Start the MCP server over stdio.
	stderr := logs.NewRing(m.Config().MaxLogBytes)
	j.Emit("mcp_start", EvStart, "Starting MCP server", nil)
	client, err := mcp.Start(ctx, mcp.StartOptions{
		Args:    tokens,
		WorkDir: workDir,
		Env:     buildEnv(p.Env),
		Stderr:  stderr,
	})
	if err != nil {
		return fmt.Errorf("%w (stderr: %s)", err, stderr.String())
	}
	session := mcp.NewSession(client)
	j.setSession(session)

	// 4-7. Initialize + tools/list with a bounded handshake timeout.
	bootCtx, cancelBoot := context.WithTimeout(ctx, m.Config().StartupTimeout())
	tools, err := session.Bootstrap(bootCtx)
	cancelBoot()
	if err != nil {
		return fmt.Errorf("mcp handshake failed: %w (stderr: %s)", err, stderr.String())
	}
	j.Emit("mcp_initialize", EvOK, "MCP initialize succeeded", nil)
	j.Emit("tools_list", EvOK, fmt.Sprintf("Found %d tools", len(tools)), map[string]any{"count": len(tools)})

	// Record tools in the job result so GET /v1/jobs/{id} can report them.
	j.setResult(map[string]any{"tools": tools})
	j.Emit("ready", EvOK, "Sandbox ready", nil)

	// 8-10. Keep the process alive until the TTL expires or cancellation.
	// Cleanup of the session/process and temp dir is handled by Manager.finish.
	<-ctx.Done()
	return nil
}

func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
