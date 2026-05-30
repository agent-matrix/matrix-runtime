package runtime

import (
	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/models"
)

// Capabilities is the payload returned by GET /v1/capabilities.
type Capabilities struct {
	Capabilities []string        `json:"capabilities"`
	Runtimes     models.Runtimes `json:"runtimes"`
	Limits       Limits          `json:"limits"`
}

// Limits reports the runtime's configured limits.
type Limits struct {
	MaxTTLSeconds     int `json:"max_ttl_seconds"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
}

// SupportedCapabilities is the static list of job capabilities advertised.
var SupportedCapabilities = []string{
	"mcp.test",
	"mcp.run",
	"model.inspect",
	"model.pull",
	"agent.run",
	"tool.run",
}

// BuildCapabilities assembles the capability report from config and host probes.
func BuildCapabilities(cfg *config.Config) Capabilities {
	return Capabilities{
		Capabilities: SupportedCapabilities,
		Runtimes:     models.DetectRuntimes(),
		Limits: Limits{
			MaxTTLSeconds:     cfg.MaxTTLSeconds,
			MaxConcurrentJobs: cfg.MaxConcurrentJobs,
		},
	}
}
