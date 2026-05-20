package aiclient

import (
	"context"
)

// EmbeddingClient 定义文本向量化 provider 适配器需要实现的接口。
type EmbeddingClient interface {
	Provider() Provider
	Embed(ctx context.Context, text string, target *ModelTarget) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string, target *ModelTarget) ([][]float32, error)
}
