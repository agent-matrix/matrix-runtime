// Package jobs implements the runtime's unit of work: a typed, TTL-bounded
// task that streams lifecycle events and produces a result. Sandboxes are a
// thin alias over jobs of type mcp.test.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/logs"
	"github.com/agent-matrix/matrix-runtime/internal/mcp"
)

// Status is the coarse lifecycle state of a job.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusComplete  Status = "complete"
	StatusError     Status = "error"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// Supported job types.
const (
	TypeMCPTest      = "mcp.test"
	TypeMCPRun       = "mcp.run"
	TypeModelInspect = "model.inspect"
	TypeModelPull    = "model.pull"
	TypeModelAttach  = "model.attach"
	TypeModelPreload = "model.preload"
	TypeAgentRun     = "agent.run"
	TypeToolRun      = "tool.run"

	TypeMatrixShellInstall = "matrixshell.install"
)

// Job is a single unit of work.
type Job struct {
	ID        string
	Type      string
	Payload   json.RawMessage
	CreatedAt time.Time
	ExpiresAt time.Time
	TTL       time.Duration

	mu      sync.Mutex
	status  Status
	result  any
	errMsg  string
	session *mcp.Session

	bus    *logs.Bus
	cancel context.CancelFunc
}

// Snapshot is the JSON view of a job returned by the API.
type Snapshot struct {
	JobID     string `json:"job_id"`
	Type      string `json:"type"`
	Status    Status `json:"status"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	Result    any    `json:"result"`
	Error     string `json:"error,omitempty"`
}

// Emit publishes a lifecycle event for the job.
func (j *Job) Emit(step, status, message string, data map[string]any) {
	j.bus.Publish(logs.Event{Step: step, Status: status, Message: message, Data: data})
}

// Bus returns the job's event bus.
func (j *Job) Bus() *logs.Bus { return j.bus }

// Status returns the current status.
func (j *Job) Status() Status {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

func (j *Job) setStatus(s Status) {
	j.mu.Lock()
	j.status = s
	j.mu.Unlock()
}

func (j *Job) setResult(r any) {
	j.mu.Lock()
	j.result = r
	j.mu.Unlock()
}

func (j *Job) setError(msg string) {
	j.mu.Lock()
	j.errMsg = msg
	j.mu.Unlock()
}

// setSession attaches a live MCP session (used by mcp.test sandboxes).
func (j *Job) setSession(s *mcp.Session) {
	j.mu.Lock()
	j.session = s
	j.mu.Unlock()
}

// Session returns the live MCP session, if any.
func (j *Job) Session() *mcp.Session {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.session
}

// decodePayload unmarshals a job payload into dst, tolerating an empty payload.
func decodePayload(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return nil
}

// Snapshot returns the current JSON view of the job.
func (j *Job) Snapshot() Snapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return Snapshot{
		JobID:     j.ID,
		Type:      j.Type,
		Status:    j.status,
		CreatedAt: j.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt: j.ExpiresAt.UTC().Format(time.RFC3339),
		Result:    j.result,
		Error:     j.errMsg,
	}
}
