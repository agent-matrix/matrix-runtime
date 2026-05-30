// Package cache manages on-disk layout for models, artifacts and job scratch
// space under the runtime data directory.
package cache

import (
	"os"
	"path/filepath"
	"strings"
)

// Layout resolves filesystem paths under a base data directory.
//
//	<data>/
//	├── models/huggingface/<ns>--<name>/{metadata.json,lock.json,snapshots/}
//	├── mcp/
//	├── agents/
//	├── jobs/
//	└── logs/
type Layout struct {
	Base string
}

// New returns a Layout rooted at base.
func New(base string) *Layout { return &Layout{Base: base} }

// Models returns the models root directory.
func (l *Layout) Models() string { return filepath.Join(l.Base, "models") }

// HuggingFace returns the Hugging Face cache root.
func (l *Layout) HuggingFace() string { return filepath.Join(l.Models(), "huggingface") }

// MCP returns the MCP scratch root.
func (l *Layout) MCP() string { return filepath.Join(l.Base, "mcp") }

// Agents returns the agents root.
func (l *Layout) Agents() string { return filepath.Join(l.Base, "agents") }

// Jobs returns the jobs root.
func (l *Layout) Jobs() string { return filepath.Join(l.Base, "jobs") }

// Logs returns the logs root.
func (l *Layout) Logs() string { return filepath.Join(l.Base, "logs") }

// ModelDir returns the cache directory for a single Hugging Face model. The
// namespace and name are joined with "--" to keep a flat, filesystem-safe path.
func (l *Layout) ModelDir(namespace, name string) string {
	return filepath.Join(l.HuggingFace(), SafeModelKey(namespace, name))
}

// SafeModelKey builds the on-disk key for a model, e.g. "Qwen--Qwen2.5-7B-Instruct".
func SafeModelKey(namespace, name string) string {
	key := namespace + "--" + name
	key = strings.ReplaceAll(key, "/", "--")
	return key
}

// EnsureDirs creates all standard subdirectories under the base.
func (l *Layout) EnsureDirs() error {
	for _, d := range []string{l.HuggingFace(), l.MCP(), l.Agents(), l.Jobs(), l.Logs()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
