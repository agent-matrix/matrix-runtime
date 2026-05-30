package models

import "context"

// PreloadVLLM serves a model with vLLM. Stubbed for the MVP.
func PreloadVLLM(_ context.Context, _ string) error {
	return ErrNotImplemented
}
