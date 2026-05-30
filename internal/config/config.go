// Package config holds runtime configuration for matrix-runtime.
//
// Configuration is sourced from environment variables and command-line flags.
// Defaults are mode-aware: the hf-space mode is intentionally constrained to a
// single concurrent job, while customer-agent allows more parallelism.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Version is the matrix-runtime release version reported by the API. It may be
// overridden at build time via -ldflags "-X .../internal/config.Version=...".
var (
	Version = "0.1.0"
	Commit  = "none"
	Date    = "unknown"
)

// Mode is a runtime operating mode. See docs/architecture.md.
type Mode string

const (
	// ModeCloudWorker is a MatrixHub-owned execution worker.
	ModeCloudWorker Mode = "cloud-worker"
	// ModeCustomerAgent runs inside customer infrastructure and dials out to the cloud.
	ModeCustomerAgent Mode = "customer-agent"
	// ModeHFSpace is a lightweight Hugging Face Space mode for short sandboxes.
	ModeHFSpace Mode = "hf-space"
	// ModeLocalDev is a developer workstation mode.
	ModeLocalDev Mode = "local-dev"
)

// Valid reports whether m is a recognised mode.
func (m Mode) Valid() bool {
	switch m {
	case ModeCloudWorker, ModeCustomerAgent, ModeHFSpace, ModeLocalDev:
		return true
	default:
		return false
	}
}

// Config is the fully-resolved runtime configuration.
type Config struct {
	Mode      Mode
	RuntimeID string
	Workspace string

	Port    int
	DataDir string

	// Limits
	MaxTTLSeconds         int
	MaxConcurrentJobs     int
	InstallTimeoutSeconds int
	StartupTimeoutSeconds int
	RPCTimeoutSeconds     int
	MaxLogBytes           int

	// Retention / cleanup. Terminal jobs and their on-disk scratch older than
	// JobRetentionHours are purged every CleanupIntervalMinutes; persisted log
	// files older than LogRetentionHours are pruned.
	JobRetentionHours      int
	LogRetentionHours      int
	CleanupIntervalMinutes int

	// RateLimitRPM caps requests per minute per client IP on write/auth
	// endpoints (0 disables). Protects against accidental abuse and brute force.
	RateLimitRPM int

	// Control plane / hybrid cloud
	CloudURL  string
	JoinToken string

	// API auth: when set, requests must present this bearer token. In production
	// modes (anything but local-dev) a missing token is unsafe and surfaced as a
	// readiness warning.
	APIToken string

	// MatrixShellEnabled gates the MatrixShell endpoints. Off by default in
	// production modes (it executes commands in a local sandbox); defaults on in
	// local-dev for developer convenience. Override with MATRIX_SHELL_ENABLED.
	MatrixShellEnabled bool

	// PublicURL is the externally reachable base URL (used in links/emails).
	PublicURL string

	// Hugging Face
	HFToken    string
	HFCacheDir string

	// Multitenant user store. By default a local SQLite file (DBPath). When
	// DatabaseURL is set, MatrixCloud uses PostgreSQL/Neon instead and isolates
	// all of its objects in DBSchema (so the instance can be shared with other
	// apps — e.g. admin.matrixhub.io — without collisions).
	DBPath      string
	DatabaseURL string
	DBSchema    string
}

// RuntimeID falls back to a mode-derived default when unset.
func (c *Config) EffectiveRuntimeID() string {
	if c.RuntimeID != "" {
		return c.RuntimeID
	}
	switch c.Mode {
	case ModeLocalDev:
		return "rt_local"
	case ModeHFSpace:
		return "rt_hf_space"
	default:
		return "rt_" + strings.ReplaceAll(string(c.Mode), "-", "_")
	}
}

// IsProduction reports whether the runtime is in a non-developer mode where
// security defaults (API token required, MatrixShell off) should be enforced.
func (c *Config) IsProduction() bool { return c.Mode != ModeLocalDev }

// MaxTTL returns the maximum sandbox/job TTL as a duration.
func (c *Config) MaxTTL() time.Duration { return time.Duration(c.MaxTTLSeconds) * time.Second }

// InstallTimeout returns the dependency-install timeout.
func (c *Config) InstallTimeout() time.Duration {
	return time.Duration(c.InstallTimeoutSeconds) * time.Second
}

// StartupTimeout returns the process startup timeout.
func (c *Config) StartupTimeout() time.Duration {
	return time.Duration(c.StartupTimeoutSeconds) * time.Second
}

// RPCTimeout returns the per-RPC timeout for MCP calls.
func (c *Config) RPCTimeout() time.Duration {
	return time.Duration(c.RPCTimeoutSeconds) * time.Second
}

// Validate checks the configuration for obvious errors.
func (c *Config) Validate() error {
	if !c.Mode.Valid() {
		return fmt.Errorf("invalid mode %q (want cloud-worker|customer-agent|hf-space|local-dev)", c.Mode)
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d", c.Port)
	}
	if c.MaxTTLSeconds <= 0 {
		return fmt.Errorf("max_ttl_seconds must be positive")
	}
	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("max_concurrent_jobs must be positive")
	}
	return nil
}
