package models

import (
	"context"
	"errors"
)

// ErrNotImplemented marks runtime preload paths that are stubbed for the MVP.
var ErrNotImplemented = errors.New("not implemented in this runtime build")

// PreloadOllama loads a model into a local Ollama daemon. Stubbed for the MVP.
func PreloadOllama(_ context.Context, _ string) error {
	return ErrNotImplemented
}
