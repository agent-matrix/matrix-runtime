package models

import (
	"context"

	"github.com/agent-matrix/matrix-runtime/internal/hf"
)

// Inspect resolves and inspects a model identifier. Only Hugging Face
// (hf:namespace/name) identifiers are supported in the MVP.
func Inspect(ctx context.Context, model, revision, hfToken string) (*hf.Metadata, error) {
	ref, err := hf.ParseRef(model, revision)
	if err != nil {
		return nil, err
	}
	client := hf.NewClient(hfToken)
	return client.Inspect(ctx, ref)
}
