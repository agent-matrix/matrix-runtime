package jobs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJanitorPurgesOldTerminalJobs(t *testing.T) {
	m := testManager(t)
	defer m.Shutdown()
	m.cfg.JobRetentionHours = 1

	// An old terminal job (with on-disk scratch) should be purged.
	old := &Job{ID: "j_old", Type: TypeModelInspect, CreatedAt: time.Now().Add(-2 * time.Hour)}
	old.setStatus(StatusComplete)
	m.store.put(old)
	scratch, _ := m.layout.MCPScratchDir("j_old")
	if _, err := os.Stat(scratch); err != nil {
		t.Fatalf("scratch dir not created: %v", err)
	}

	// A recent terminal job and an old-but-running job must survive.
	recent := &Job{ID: "j_recent", Type: TypeModelInspect, CreatedAt: time.Now()}
	recent.setStatus(StatusComplete)
	m.store.put(recent)
	running := &Job{ID: "j_running", Type: TypeModelInspect, CreatedAt: time.Now().Add(-3 * time.Hour)}
	running.setStatus(StatusRunning)
	m.store.put(running)

	m.runCleanup()

	if _, ok := m.store.get("j_old"); ok {
		t.Error("old terminal job should have been purged")
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Errorf("scratch dir for purged job should be gone, stat err=%v", err)
	}
	if _, ok := m.store.get("j_recent"); !ok {
		t.Error("recent job should survive")
	}
	if _, ok := m.store.get("j_running"); !ok {
		t.Error("running job should never be purged")
	}
}

func TestJanitorPrunesOldLogFiles(t *testing.T) {
	m := testManager(t)
	defer m.Shutdown()
	m.cfg.LogRetentionHours = 1

	logsDir := m.layout.Logs()
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldLog := filepath.Join(logsDir, "old.log")
	newLog := filepath.Join(logsDir, "new.log")
	_ = os.WriteFile(oldLog, []byte("x"), 0o644)
	_ = os.WriteFile(newLog, []byte("y"), 0o644)
	old := time.Now().Add(-2 * time.Hour)
	_ = os.Chtimes(oldLog, old, old)

	m.runCleanup()

	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Errorf("old log should be pruned, stat err=%v", err)
	}
	if _, err := os.Stat(newLog); err != nil {
		t.Errorf("recent log should survive: %v", err)
	}
}
