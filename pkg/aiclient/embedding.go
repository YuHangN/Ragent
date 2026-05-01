package aiclient

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

// EmbeddingService converts text to dense float vectors.
type EmbeddingService interface {
	// Embed embeds a single text. Internally calls EmbedBatch.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch embeds a batch of texts. Returns a slice of float32 vectors.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the vector size for schema validation (e.g. 1024, 1536).
	Dimension() int
}

type routingEmbeddingService struct {
	selector         *Selector
	healthStore      *HealthStore
	clientByProvider map[Provider]EmbeddingClient
	dimension        int
}

func NewEmbeddingService(cfg *config.AIConfig, hs *HealthStore, clients []EmbeddingClient) (EmbeddingService, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("embedding: at least one EmbeddingClient required")
	}
	clientMap := make(map[Provider]EmbeddingClient, len(clients))
	for _, c := range clients {
		clientMap[c.Provider()] = c
	}
	selector := NewSelector(cfg, hs)
	def := selector.SelectDefaultEmbedding()
	dim := 0
	if def != nil {
		dim = def.Candidate.Dimension
	}
	return &routingEmbeddingService{
		selector:         selector,
		healthStore:      hs,
		clientByProvider: clientMap,
		dimension:        dim,
	}, nil
}

func (s *routingEmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
	out, err := s.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

func (s *routingEmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	targets := s.selector.SelectEmbeddingCandidates()
	return ExecuteWithFallback(
		s.healthStore,
		CapabilityEmbedding,
		targets,
		func(t *ModelTarget) EmbeddingClient {
			return s.clientByProvider[Provider(t.Candidate.Provider)]
		},
		func(c EmbeddingClient, t *ModelTarget) ([][]float32, error) {
			return c.EmbedBatch(ctx, texts, t)
		},
	)
}

func (s *routingEmbeddingService) Dimension() int { return s.dimension }
