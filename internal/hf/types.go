// Package hf resolves and inspects Hugging Face model identifiers. It refactors
// the model-resolution and metadata-parsing ideas from the legacy
// matrixhub-ai HF discovery code into a small, standalone client.
package hf

import "fmt"

// HumanParams formats a parameter count as a short string (e.g. 7600000000 -> "7.6B").
func HumanParams(n int64) string {
	switch {
	case n <= 0:
		return "—"
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.0fM", float64(n)/1e6)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// Ref is a parsed Hugging Face model reference.
type Ref struct {
	Source    string `json:"source"`    // always "huggingface"
	Namespace string `json:"namespace"` // e.g. "Qwen"
	Name      string `json:"name"`      // e.g. "Qwen2.5-7B-Instruct"
	Revision  string `json:"revision"`  // e.g. "main"
}

// RepoID returns the canonical "namespace/name" identifier.
func (r Ref) RepoID() string { return r.Namespace + "/" + r.Name }

// Metadata is the subset of Hugging Face model metadata that the runtime cares
// about for inspection and runtime-recommendation purposes.
type Metadata struct {
	Model               string   `json:"model"`
	Source              string   `json:"source"`
	Namespace           string   `json:"namespace"`
	Name                string   `json:"name"`
	Revision            string   `json:"revision"`
	PipelineTag         string   `json:"pipeline_tag,omitempty"`
	LibraryName         string   `json:"library_name,omitempty"`
	License             string   `json:"license,omitempty"`
	ModelType           string   `json:"model_type,omitempty"`
	EstimatedParameters int64    `json:"estimated_parameters,omitempty"`
	RecommendedRuntime  string   `json:"recommended_runtime,omitempty"`
	RequiresGPU         bool     `json:"requires_gpu"`
	Tags                []string `json:"tags,omitempty"`
	Gated               bool     `json:"gated,omitempty"`
	Private             bool     `json:"private,omitempty"`
}
