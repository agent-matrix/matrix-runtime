package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/cache"
	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/logs"
	"github.com/agent-matrix/matrix-runtime/internal/security"
)

// Handler executes a job. It must honour ctx cancellation and emit lifecycle
// events via j.Emit. The terminal status is derived by the manager from the
// returned error and ctx state.
type Handler func(ctx context.Context, m *Manager, j *Job) error

// CreateRequest is the input for creating a job.
type CreateRequest struct {
	Type       string          `json:"type"`
	TTLSeconds int             `json:"ttl_seconds"`
	Payload    json.RawMessage `json:"payload"`
}

// InstallStore is the subset of the user store the model.attach job needs to
// persist installation progress. It is satisfied by *store.Store.
type InstallStore interface {
	UpdateInstallation(id, status string, progress int, localPath, endpointURL string) error
	SetProfileStatus(id, status string) error
}

// Manager owns the job lifecycle, concurrency limiting and handler dispatch.
type Manager struct {
	cfg      *config.Config
	layout   *cache.Layout
	store    *store
	db       InstallStore
	handlers map[string]Handler
	sem      chan struct{}

	baseCtx context.Context
	cancel  context.CancelFunc
}

// SetInstallStore attaches the persistent installation store used by
// model.attach. Optional: when nil, attach jobs run without persistence.
func (m *Manager) SetInstallStore(db InstallStore) { m.db = db }

// InstallDB returns the attached installation store (may be nil).
func (m *Manager) InstallDB() InstallStore { return m.db }

// ErrUnknownType is returned when a job type has no registered handler.
var ErrUnknownType = errors.New("unknown job type")

// NewManager builds a Manager and registers the built-in handlers.
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		cfg:      cfg,
		layout:   cache.New(cfg.DataDir),
		store:    newStore(),
		handlers: make(map[string]Handler),
		sem:      make(chan struct{}, cfg.MaxConcurrentJobs),
		baseCtx:  ctx,
		cancel:   cancel,
	}
	m.handlers[TypeMCPTest] = handleMCPTest
	m.handlers[TypeMCPRun] = handleMCPRun
	m.handlers[TypeModelInspect] = handleModelInspect
	m.handlers[TypeModelPull] = handleModelPull
	m.handlers[TypeModelAttach] = handleModelAttach
	m.handlers[TypeModelPreload] = handleModelPreload
	m.handlers[TypeAgentRun] = handleAgentRun
	m.handlers[TypeToolRun] = handleToolRun
	m.handlers[TypeMatrixShellInstall] = handleMatrixShellInstall
	return m
}

// Config exposes the runtime configuration to handlers and the API.
func (m *Manager) Config() *config.Config { return m.cfg }

// Layout exposes the cache layout to handlers.
func (m *Manager) Layout() *cache.Layout { return m.layout }

// Create validates and starts a new job, returning its snapshot.
func (m *Manager) Create(req CreateRequest) (*Job, error) {
	if _, ok := m.handlers[req.Type]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, req.Type)
	}
	if err := security.CheckTTL(req.TTLSeconds, m.cfg.MaxTTLSeconds); err != nil {
		return nil, err
	}
	ttl := security.ClampTTL(req.TTLSeconds, m.cfg.MaxTTLSeconds)

	now := time.Now().UTC()
	jobCtx, cancel := context.WithTimeout(m.baseCtx, ttl)
	j := &Job{
		ID:        newID("job_"),
		Type:      req.Type,
		Payload:   req.Payload,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		TTL:       ttl,
		status:    StatusQueued,
		bus:       logs.NewBus(),
		cancel:    cancel,
	}
	m.store.put(j)
	j.Emit("queue", EvQueued, "Job queued", map[string]any{"type": j.Type})

	go m.run(jobCtx, j)
	return j, nil
}

func (m *Manager) run(ctx context.Context, j *Job) {
	defer j.cancel()

	// Acquire a concurrency slot, respecting cancellation.
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	case <-ctx.Done():
		m.finish(j, ctx, errors.New("cancelled before start"))
		return
	}

	j.setStatus(StatusRunning)
	handler := m.handlers[j.Type]
	err := handler(ctx, m, j)
	m.finish(j, ctx, err)
}

// finish derives and records the terminal status, emits a final event and
// closes the event bus.
func (m *Manager) finish(j *Job, ctx context.Context, err error) {
	// Clean up any live MCP session and scratch space.
	if s := j.Session(); s != nil {
		s.Close()
	}
	m.layout.RemoveJob(j.ID)

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		j.setStatus(StatusExpired)
		j.Emit("expire", EvExpired, fmt.Sprintf("Job expired after %d seconds", int(j.TTL.Seconds())), nil)
	case errors.Is(ctx.Err(), context.Canceled):
		j.setStatus(StatusCancelled)
		j.Emit("cancel", EvCancelled, "Job cancelled", nil)
	case err != nil:
		j.setStatus(StatusError)
		j.setError(err.Error())
		j.Emit("error", EvError, err.Error(), nil)
	default:
		j.setStatus(StatusComplete)
		j.Emit("complete", EvComplete, "Job complete", nil)
	}
	j.Bus().Close()
}

// Get returns a job by id.
func (m *Manager) Get(id string) (*Job, bool) { return m.store.get(id) }

// List returns snapshots of all jobs, newest first.
func (m *Manager) List() []Snapshot {
	jobs := m.store.list()
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	out := make([]Snapshot, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.Snapshot())
	}
	return out
}

// Cancel cancels a running job. It returns false when the job is unknown.
func (m *Manager) Cancel(id string) (Status, bool) {
	j, ok := m.store.get(id)
	if !ok {
		return "", false
	}
	j.cancel()
	// Give the run goroutine a moment to record the terminal state.
	for i := 0; i < 50; i++ {
		switch j.Status() {
		case StatusCancelled, StatusExpired, StatusComplete, StatusError:
			return j.Status(), true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return StatusCancelled, true
}

// Shutdown cancels all jobs and waits briefly for cleanup.
func (m *Manager) Shutdown() {
	m.cancel()
	time.Sleep(200 * time.Millisecond)
}

func newID(prefix string) string {
	var b [9]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
