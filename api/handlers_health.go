package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/runtime"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"runtime_id": s.cfg.EffectiveRuntimeID(),
		"mode":       string(s.cfg.Mode),
		"version":    config.Version,
	})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, runtime.BuildCapabilities(s.cfg))
}

// handleVersion returns build/version metadata for monitoring and support.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":       "matrix-runtime",
		"version":    config.Version,
		"commit":     config.Commit,
		"build_time": config.Date,
		"mode":       string(s.cfg.Mode),
	})
}

// handleReady is the readiness probe: it verifies core dependencies (database
// reachable, data directory writable, job manager running, mode valid) and
// reports non-fatal misconfiguration as warnings. It returns 200 when ready and
// 503 when a core check fails, so Kubernetes can gate traffic.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	checks := map[string]bool{}
	warnings := []map[string]string{}
	warn := func(code, msg string) { warnings = append(warnings, map[string]string{"code": code, "message": msg}) }

	// Mode valid.
	checks["mode_valid"] = s.cfg.Mode.Valid()

	// Job manager running.
	checks["job_manager"] = s.manager != nil

	// Data directory writable.
	checks["data_dir_writable"] = dirWritable(s.cfg.DataDir)

	// Database reachable (only when a store is configured).
	if s.store != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		checks["database"] = s.store.Ping(ctx) == nil
	} else {
		checks["database"] = false
		warn("store_unavailable", "User store is not available; auth endpoints return 503.")
	}

	// Non-fatal security warnings.
	if s.cfg.IsProduction() && s.cfg.APIToken == "" {
		warn("api_token_missing", "MATRIX_RUNTIME_API_TOKEN is not set; the API is unauthenticated for non-session callers.")
	}
	if s.cfg.MatrixShellEnabled {
		warn("matrixshell_enabled", "MatrixShell is enabled; it executes commands in a local sandbox.")
	}
	if s.cfg.IsProduction() && s.cfg.DatabaseURL == "" {
		warn("sqlite_in_use", "Using SQLite; configure MATRIX_RUNTIME_DATABASE_URL (Postgres) for multi-user/HA deployments.")
	}
	if s.cfg.Mode == config.ModeLocalDev {
		warn("local_dev_mode", "Runtime is in local-dev mode; not hardened for production.")
	}

	ready := true
	for _, ok := range checks {
		if !ok {
			ready = false
		}
	}
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"ready": ready, "checks": checks, "warnings": warnings})
}

// dirWritable reports whether dir exists (creating it if needed) and accepts a
// temporary file write.
func dirWritable(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".ready-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(filepath.Join(dir, filepath.Base(name)))
	return true
}
