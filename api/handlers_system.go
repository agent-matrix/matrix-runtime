package api

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
)

// dirSize sums the size of all regular files under dir (0 when missing).
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func fileSize(path string) int64 {
	if info, err := os.Stat(path); err == nil {
		return info.Size()
	}
	return 0
}

// handleStorage reports disk usage for the runtime's data directory: per-area
// sizes, the database file, job count, and free space on the filesystem.
func (s *Server) handleStorage(w http.ResponseWriter, _ *http.Request) {
	l := s.manager.Layout()
	areas := map[string]int64{
		"models":   dirSize(l.HuggingFace()),
		"mcp":      dirSize(l.MCP()),
		"agents":   dirSize(l.Agents()),
		"jobs":     dirSize(l.Jobs()),
		"logs":     dirSize(l.Logs()),
		"database": fileSize(s.cfg.DBPath),
	}
	var total int64
	for _, v := range areas {
		total += v
	}

	jobsCount := 0
	if entries, err := os.ReadDir(l.Jobs()); err == nil {
		jobsCount = len(entries)
	}

	var freeBytes uint64
	var fsStat syscall.Statfs_t
	if err := syscall.Statfs(s.cfg.DataDir, &fsStat); err == nil {
		freeBytes = fsStat.Bavail * uint64(fsStat.Bsize)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data_dir":    s.cfg.DataDir,
		"areas":       areas,
		"total_bytes": total,
		"free_bytes":  freeBytes,
		"jobs_count":  jobsCount,
	})
}
