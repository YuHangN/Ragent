package aiclient

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

// EmbeddingService 是业务侧使用的文本向量化入口。
type EmbeddingService interface {
	// Embed 对单段文本生成向量。
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch 对多段文本批量生成向量。
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension 返回当前默认向量模型的维度，用于 schema 校验。
	Dimension() int
}

// routingEmbeddingService 是带候选选择、熔断和 fallback 的 EmbeddingService 实现。
type routingEmbeddingService struct {
	selector         *Selector
	healthStore      *HealthStore
	clientByProvider map[Provider]EmbeddingClient
	dimension        int
}

// NewEmbeddingService 构造路由式 EmbeddingService。
//
// clients 中每个 EmbeddingClient 的 Provider() 会作为路由表 key。
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

// Embed 对单段文本生成向量。
func (s *routingEmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
	out, err := s.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// EmbedBatch 选择可用向量化模型并批量生成向量。
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

// Dimension 返回默认向量化候选的维度。
func (s *routingEmbeddingService) Dimension() int { return s.dimension }
