package ingestion

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// EmbedderNode calls EmbeddingService.EmbedBatch on all chunk contents
// and stores the resulting vectors back into each VectorChunk.Embedding.

type EmbedderNode struct {
	embedding aiclient.EmbeddingService
}

func NewEmbedderNode(embedding aiclient.EmbeddingService) *EmbedderNode {
	return &EmbedderNode{embedding: embedding}
}

func (n *EmbedderNode) Name() string { return "embedder" }

func (n *EmbedderNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if len(ic.Chunks) == 0 {
		return Fail(fmt.Errorf("embedder: no chunks to embed"))
	}

	texts := make([]string, len(ic.Chunks))
	for i, ch := range ic.Chunks {
		texts[i] = ch.Content
	}

	vectors, err := n.embedding.EmbedBatch(ctx, texts)
	if err != nil {
		return Fail(fmt.Errorf("embedder: EmbedBatch: %w", err))
	}
	if len(vectors) != len(ic.Chunks) {
		return Fail(fmt.Errorf("embedder: expected %d vectors, got %d", len(ic.Chunks), len(vectors)))
	}

	for i := range ic.Chunks {
		ic.Chunks[i].Embedding = vectors[i]
	}
	return OK(fmt.Sprintf("embedded %d chunks (dim=%d)", len(ic.Chunks), len(vectors[0])))
}
