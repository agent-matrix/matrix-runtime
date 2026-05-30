package api

import (
	"net/http"

	"github.com/agent-matrix/matrix-runtime/internal/catalog"
	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/runtime"
	"github.com/agent-matrix/matrix-runtime/internal/security"
)

// handleListRuntimes returns the real runtimes known to this control surface.
// Today that is this node (derived from live health + capabilities); joined
// remote runtimes will appear here once the control-channel lands.
func (s *Server) handleListRuntimes(w http.ResponseWriter, _ *http.Request) {
	caps := runtime.BuildCapabilities(s.cfg)
	running := 0
	for _, j := range s.manager.List() {
		if j.Status == "running" || j.Status == "queued" {
			running++
		}
	}
	self := map[string]any{
		"id":          s.cfg.EffectiveRuntimeID(),
		"name":        s.cfg.EffectiveRuntimeID(),
		"status":      "Online",
		"statusClass": "green",
		"mode":        string(s.cfg.Mode),
		"region":      "local",
		"caps":        caps.Capabilities,
		"runtimes":    caps.Runtimes,
		"jobs":        running,
		"version":     config.Version,
		"heartbeat":   "just now",
		"live":        true,
	}
	writeJSON(w, http.StatusOK, map[string]any{"runtimes": []any{self}})
}

// handleCatalog returns the curated component catalog (real reference data).
func (s *Server) handleCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": catalog.Items})
}

// handlePolicies returns the runtime's actual enforced policy — derived from
// config limits and the security allow/deny lists (not hand-written demo JSON).
func (s *Server) handlePolicies(w http.ResponseWriter, _ *http.Request) {
	c := s.cfg
	policies := []map[string]any{
		{
			"name": "Sandbox Policy", "active": true,
			"body": map[string]any{
				"max_ttl_seconds":         c.MaxTTLSeconds,
				"max_concurrent_jobs":     c.MaxConcurrentJobs,
				"startup_timeout_seconds": c.StartupTimeoutSeconds,
				"rpc_timeout_seconds":     c.RPCTimeoutSeconds,
				"max_log_bytes":           c.MaxLogBytes,
				"allowed_programs":        security.AllowedPrograms(),
				"allowed_transports":      []string{"stdio"},
				"reject_raw_secrets":      true,
			},
		},
		{
			"name": "Command Policy", "active": true,
			"body": map[string]any{
				"blocked_tokens":          security.BlockedTokens(),
				"blocked_shell_operators": []string{"&", "|", ";", "<", ">", "`", "$()", "newline"},
			},
		},
		{
			"name": "Runtime Policy", "active": true,
			"body": map[string]any{
				"mode":                string(c.Mode),
				"api_token_set":       c.APIToken != "",
				"matrixshell_enabled": c.MatrixShellEnabled,
				"rate_limit_rpm":      c.RateLimitRPM,
				"cloud_url":           c.CloudURL,
				"outbound_only":       true,
				"data_dir":            c.DataDir,
			},
		},
	}
	writeJSON(w, http.StatusOK, map[string]any{"policies": policies})
}
