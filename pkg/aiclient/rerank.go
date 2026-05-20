package aiclient

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

// RerankService 是业务侧使用的检索结果重排入口。
type RerankService interface {
	Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int) ([]RetrievedChunk, error)
}

// routingRerankService 是带候选选择、熔断和 fallback 的 RerankService 实现。
type routingRerankService struct {
	selector         *Selector
	healthStore      *HealthStore
	clientByProvider map[Provider]RerankClient
}

// NewRerankService 构造路由式 RerankService。
//
// clients 中每个 RerankClient 的 Provider() 会作为路由表 key。
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

// Rerank 选择可用重排模型，并返回按相关性重排后的片段。
func (s *routingRerankService) Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int) ([]RetrievedChunk, error) {
	targets := s.selector.SelectRerankCandidates()
	if len(targets) == 0 {
		// 配置里没有 rerank 候选时，优先用 NoopRerankClient 做本地截断。
		if noop, ok := s.clientByProvider[ProviderNoop]; ok {
			return noop.Rerank(ctx, query, candidates, topN, nil)
		}
		// 没注册 noop client 时仍保持可用：按原顺序本地截断。
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
