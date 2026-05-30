package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agent-matrix/matrix-runtime/internal/mcp"
)

// SandboxRequest is the input for creating a sandbox session. It is a thin
// alias over an mcp.test job.
type SandboxRequest struct {
	EntityID     string            `json:"entity_id"`
	TTLSeconds   int               `json:"ttl_seconds"`
	Runtime      string            `json:"runtime"`
	Transport    string            `json:"transport"`
	StartCommand string            `json:"start_command"`
	Env          map[string]string `json:"env,omitempty"`
}

// CreateSandbox starts an mcp.test job and registers a sandbox session id that
// maps to it. It returns the session id and the underlying job.
func (m *Manager) CreateSandbox(req SandboxRequest) (string, *Job, error) {
	payload, err := json.Marshal(mcpTestPayload{
		Runtime:      req.Runtime,
		Transport:    req.Transport,
		StartCommand: req.StartCommand,
		Env:          req.Env,
	})
	if err != nil {
		return "", nil, err
	}
	j, err := m.Create(CreateRequest{
		Type:       TypeMCPTest,
		TTLSeconds: req.TTLSeconds,
		Payload:    payload,
	})
	if err != nil {
		return "", nil, err
	}
	sessionID := newID("sbx_")
	m.store.linkSandbox(sessionID, j.ID)
	return sessionID, j, nil
}

// SandboxJob returns the job backing a sandbox session.
func (m *Manager) SandboxJob(sessionID string) (*Job, bool) {
	return m.store.jobForSandbox(sessionID)
}

// SandboxTools returns the tools advertised by a sandbox's MCP server.
func (m *Manager) SandboxTools(sessionID string) ([]mcp.Tool, error) {
	j, ok := m.store.jobForSandbox(sessionID)
	if !ok {
		return nil, fmt.Errorf("sandbox session not found")
	}
	s := j.Session()
	if s == nil || !s.Ready() {
		return nil, fmt.Errorf("sandbox not ready")
	}
	return s.Tools(), nil
}

// CallSandboxTool invokes a tool within a running sandbox session.
func (m *Manager) CallSandboxTool(ctx context.Context, sessionID, name string, args map[string]any) (json.RawMessage, error) {
	j, ok := m.store.jobForSandbox(sessionID)
	if !ok {
		return nil, fmt.Errorf("sandbox session not found")
	}
	s := j.Session()
	if s == nil || !s.Ready() {
		return nil, fmt.Errorf("sandbox not ready")
	}
	callCtx, cancel := context.WithTimeout(ctx, m.Config().RPCTimeout())
	defer cancel()
	return s.CallTool(callCtx, name, args)
}
