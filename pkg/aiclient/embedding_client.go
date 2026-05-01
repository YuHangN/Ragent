package aiclient

import (
	"context"
)

type EmbeddingClient interface {
	Provider() Provider
	Embed(ctx context.Context, text string, target *ModelTarget) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string, target *ModelTarget) ([][]float32, error)
}
