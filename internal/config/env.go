package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Defaults returns a Config populated with mode-aware defaults.
func Defaults(mode Mode) *Config {
	if !mode.Valid() {
		mode = ModeLocalDev
	}
	maxConcurrent := 2
	switch mode {
	case ModeHFSpace:
		maxConcurrent = 1
	case ModeCustomerAgent, ModeCloudWorker:
		maxConcurrent = 5
	}
	dataDir := "/var/lib/matrix-runtime"
	if mode == ModeLocalDev {
		if home, err := os.UserHomeDir(); err == nil {
			dataDir = filepath.Join(home, ".matrix", "runtime", "data")
		}
	}
	return &Config{
		Mode:                   mode,
		Port:                   8080,
		DataDir:                dataDir,
		MaxTTLSeconds:          600,
		MaxConcurrentJobs:      maxConcurrent,
		InstallTimeoutSeconds:  120,
		StartupTimeoutSeconds:  45,
		RPCTimeoutSeconds:      20,
		MaxLogBytes:            1024 * 1024,
		CloudURL:               "https://cloud.matrixhub.io",
		HFCacheDir:             filepath.Join(dataDir, "models", "huggingface"),
		DBPath:                 filepath.Join(dataDir, "matrixcloud.db"),
		DBSchema:               "matrixcloud",
		JobRetentionHours:      24,
		LogRetentionHours:      72,
		CleanupIntervalMinutes: 15,
		RateLimitRPM:           120,
	}
}

// FromEnv builds a Config from environment variables, layered on top of the
// mode-aware defaults. The mode is resolved from modeOverride (a CLI flag) when
// non-empty, otherwise from MATRIX_RUNTIME_MODE, otherwise local-dev.
func FromEnv(modeOverride string) *Config {
	mode := Mode(firstNonEmpty(modeOverride, os.Getenv("MATRIX_RUNTIME_MODE"), string(ModeLocalDev)))
	c := Defaults(mode)

	// Honor MATRIX_RUNTIME_PORT, else the PaaS convention $PORT (Cloud Run,
	// Render, Railway, Koyeb, Heroku, …) so hosted deploys work out of the box.
	if v := firstNonEmpty(os.Getenv("MATRIX_RUNTIME_PORT"), os.Getenv("PORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Port = n
		}
	}
	if v := os.Getenv("MATRIX_RUNTIME_DATA_DIR"); v != "" {
		c.DataDir = v
		c.HFCacheDir = filepath.Join(v, "models", "huggingface")
		c.DBPath = filepath.Join(v, "matrixcloud.db")
	}
	if v := os.Getenv("MATRIX_RUNTIME_DB_PATH"); v != "" {
		c.DBPath = v
	}
	if v := os.Getenv("MATRIX_RUNTIME_MAX_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxTTLSeconds = n
		}
	}
	if v := os.Getenv("MATRIX_RUNTIME_MAX_CONCURRENT_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxConcurrentJobs = n
		}
	}
	c.RuntimeID = firstNonEmpty(os.Getenv("MATRIX_RUNTIME_ID"), c.RuntimeID)
	c.Workspace = firstNonEmpty(os.Getenv("MATRIX_RUNTIME_WORKSPACE"), c.Workspace)
	c.CloudURL = firstNonEmpty(os.Getenv("MATRIX_CLOUD_URL"), c.CloudURL)
	c.JoinToken = firstNonEmpty(os.Getenv("MATRIX_RUNTIME_JOIN_TOKEN"), c.JoinToken)
	c.APIToken = firstNonEmpty(os.Getenv("MATRIX_RUNTIME_API_TOKEN"), c.APIToken)
	c.HFToken = firstNonEmpty(os.Getenv("HF_TOKEN"), c.HFToken)
	if v := os.Getenv("MATRIX_RUNTIME_HF_CACHE_DIR"); v != "" {
		c.HFCacheDir = v
	}
	// PostgreSQL/Neon connection for the hosted control plane. Several common
	// env names are accepted; the first non-empty one wins. Never hardcode the
	// DSN — it carries credentials and must come from the environment.
	c.DatabaseURL = firstNonEmpty(
		os.Getenv("MATRIXCLOUD_DATABASE_URL"),
		os.Getenv("MATRIX_RUNTIME_DB_URL"),
		os.Getenv("DATABASE_URL"),
	)
	c.DBSchema = firstNonEmpty(os.Getenv("MATRIXCLOUD_DB_SCHEMA"), c.DBSchema, "matrixcloud")
	c.PublicURL = firstNonEmpty(os.Getenv("MATRIX_RUNTIME_PUBLIC_URL"), os.Getenv("MATRIXCLOUD_APP_URL"))
	if n, ok := envInt("MATRIX_RUNTIME_JOB_RETENTION_HOURS"); ok {
		c.JobRetentionHours = n
	}
	if n, ok := envInt("MATRIX_RUNTIME_LOG_RETENTION_HOURS"); ok {
		c.LogRetentionHours = n
	}
	if n, ok := envInt("MATRIX_RUNTIME_CLEANUP_INTERVAL_MINUTES"); ok {
		c.CleanupIntervalMinutes = n
	}
	if n, ok := envInt("MATRIX_RUNTIME_RATE_LIMIT_RPM"); ok {
		c.RateLimitRPM = n
	}

	// MatrixShell executes commands in a local sandbox: safe-by-default means it
	// is ON only in local-dev unless explicitly enabled. MATRIX_SHELL_ENABLED
	// (or MATRIX_RUNTIME_MATRIXSHELL_ENABLED) overrides in either direction.
	c.MatrixShellEnabled = mode == ModeLocalDev
	if v := firstNonEmpty(os.Getenv("MATRIX_RUNTIME_MATRIXSHELL_ENABLED"), os.Getenv("MATRIX_SHELL_ENABLED")); v != "" {
		c.MatrixShellEnabled = parseBool(v)
	}
	return c
}

// parseBool interprets common truthy/falsey strings; unknown values are false.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

// envInt reads an integer env var; ok is false when unset or unparseable.
func envInt(key string) (int, bool) {
	v := os.Getenv(key)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
