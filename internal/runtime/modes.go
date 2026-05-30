// Package runtime exposes mode metadata and capability reporting for the
// runtime API.
package runtime

import "github.com/agent-matrix/matrix-runtime/internal/config"

// AllModes returns every supported runtime mode.
func AllModes() []config.Mode {
	return []config.Mode{
		config.ModeCloudWorker,
		config.ModeCustomerAgent,
		config.ModeHFSpace,
		config.ModeLocalDev,
	}
}

// Describe returns a human-readable description of a mode.
func Describe(m config.Mode) string {
	switch m {
	case config.ModeCloudWorker:
		return "MatrixHub-owned execution worker."
	case config.ModeCustomerAgent:
		return "Runtime deployed in customer infrastructure; connects outbound to MatrixHub Cloud."
	case config.ModeHFSpace:
		return "Lightweight Hugging Face Space mode for 10-minute MCP sandbox testing."
	case config.ModeLocalDev:
		return "Developer workstation mode."
	default:
		return "Unknown mode."
	}
}
