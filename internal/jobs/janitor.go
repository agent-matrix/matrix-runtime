package jobs

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

// StartJanitor runs a background retention loop until ctx is cancelled. On each
// tick it purges terminal jobs (and their on-disk scratch) older than the
// configured job-retention window and prunes persisted log files older than the
// log-retention window. It is safe to call once at startup.
func (m *Manager) StartJanitor(ctx context.Context) {
	interval := time.Duration(m.cfg.CleanupIntervalMinutes) * time.Minute
	if interval <= 0 {
		return // cleanup disabled
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		// Run once shortly after startup so a long interval doesn't delay the
		// first sweep.
		m.runCleanup()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.runCleanup()
			}
		}
	}()
}

// runCleanup performs one retention sweep.
func (m *Manager) runCleanup() {
	now := time.Now()

	// 1) In-memory terminal jobs + their scratch dirs.
	if m.cfg.JobRetentionHours > 0 {
		cutoff := now.Add(-time.Duration(m.cfg.JobRetentionHours) * time.Hour)
		removed := m.store.purgeTerminalBefore(cutoff)
		for _, id := range removed {
			m.layout.RemoveJob(id)
		}
		// 2) Orphaned on-disk job dirs (e.g. left by a crash) past retention.
		pruneOldDirs(m.layout.Jobs(), cutoff)
		pruneOldDirs(m.layout.MCP(), cutoff)
		if len(removed) > 0 {
			log.Printf("janitor: purged %d terminal job(s) older than %dh", len(removed), m.cfg.JobRetentionHours)
		}
	}

	// 3) Persisted log files past their retention window.
	if m.cfg.LogRetentionHours > 0 {
		logCutoff := now.Add(-time.Duration(m.cfg.LogRetentionHours) * time.Hour)
		pruneOldFiles(m.layout.Logs(), logCutoff)
	}
}

// pruneOldDirs removes immediate subdirectories of root whose modification time
// is before cutoff. Missing roots are ignored.
func pruneOldDirs(root string, cutoff time.Time) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, e.Name()))
	}
}

// pruneOldFiles removes files (recursively) under root older than cutoff.
func pruneOldFiles(root string, cutoff time.Time) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
		return nil
	})
}
