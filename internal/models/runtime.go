// Package models provides model-serving runtime detection and the model
// inspection logic used by the model.inspect job.
package models

import "os/exec"

// Runtimes reports which model-serving runtimes are available on the host.
type Runtimes struct {
	Node   bool `json:"node"`
	Python bool `json:"python"`
	Ollama bool `json:"ollama"`
	VLLM   bool `json:"vllm"`
	SGLang bool `json:"sglang"`
}

// DetectRuntimes probes the host PATH for known runtimes.
func DetectRuntimes() Runtimes {
	return Runtimes{
		Node:   hasBinary("node"),
		Python: hasBinary("python3") || hasBinary("python"),
		Ollama: hasBinary("ollama"),
		VLLM:   hasBinary("vllm"),
		SGLang: hasBinary("sglang") || hasBinary("python") && false, // sglang is a python module; treated as unavailable unless CLI present
	}
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
