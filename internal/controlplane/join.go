package controlplane

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// JoinConfig is persisted to ~/.matrix/runtime/config.yaml by the join command.
type JoinConfig struct {
	CloudURL  string
	JoinToken string
	RuntimeID string
	Workspace string
}

// ConfigPath returns the on-disk join config path under the user's home.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".matrix", "runtime", "config.yaml"), nil
}

// WriteJoinConfig writes the join configuration as YAML. A minimal hand-rolled
// encoder is used to avoid a YAML dependency in the MVP.
func WriteJoinConfig(cfg JoinConfig) (string, error) {
	if cfg.CloudURL == "" || cfg.JoinToken == "" {
		return "", fmt.Errorf("cloud url and join token are required")
	}
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cloud_url: %s\n", cfg.CloudURL)
	fmt.Fprintf(&b, "join_token: %s\n", cfg.JoinToken)
	if cfg.RuntimeID != "" {
		fmt.Fprintf(&b, "runtime_id: %s\n", cfg.RuntimeID)
	}
	if cfg.Workspace != "" {
		fmt.Fprintf(&b, "workspace: %s\n", cfg.Workspace)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
