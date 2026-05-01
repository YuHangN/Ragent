package aiclient

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

type RerankService interface {
	Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int) ([]RetrievedChunk, error)
}

type routingRerankService struct {
	selector         *Selector
	healthStore      *HealthStore
	clientByProvider map[Provider]RerankClient
}

func NewRerankService(cfg *config.AIConfig, hs *HealthStore, clients []RerankClient) (RerankService, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("rerank: at least one RerankClient required")
	}
	clientMap := make(map[Provider]RerankClient, len(clients))
	for _, c := range clients {
		clientMap[c.Provider()] = c
	}
	return &routingRerankService{
		selector:         NewSelector(cfg, hs),
		healthStore:      hs,
		clientByProvider: clientMap,
	}, nil
}

func (s *routingRerankService) Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int) ([]RetrievedChunk, error) {
	targets := s.selector.SelectRerankCandidates()
	if len(targets) == 0 {
		// 配置里没 rerank 候选 → 用 NoopRerankClient 直接截断
		if noop, ok := s.clientByProvider[ProviderNoop]; ok {
			return noop.Rerank(ctx, query, candidates, topN, nil)
		}
		// 兜底：原地截断
		if topN > 0 && topN < len(candidates) {
			return candidates[:topN], nil
		}
		return candidates, nil
	}
	return ExecuteWithFallback(
		s.healthStore,
		CapabilityRerank,
		targets,
		func(t *ModelTarget) RerankClient {
			return s.clientByProvider[Provider(t.Candidate.Provider)]
		},
		func(c RerankClient, t *ModelTarget) ([]RetrievedChunk, error) {
			return c.Rerank(ctx, query, candidates, topN, t)
		},
	)
}
