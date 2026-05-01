package aiclient

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

// LLMService 业务侧使用的 LLM 入口。接口保持稳定，实现从单候选升级为路由 + fallback。
type LLMService interface {
	Chat(ctx context.Context, req ChatRequest) (string, error)
	StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback) error
}

// routingLLMService 路由式实现。对齐 Java RoutingLLMService。
type routingLLMService struct {
	selector         *Selector
	healthStore      *HealthStore
	clientByProvider map[Provider]ChatClient
}

// NewLLMService 构造路由 LLM Service。clients 列表里每个 client 的 Provider() 用作路由 key。
func NewLLMService(cfg *config.AIConfig, hs *HealthStore, clients []ChatClient) (LLMService, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("llm: at least one ChatClient required")
	}
	clientMap := make(map[Provider]ChatClient, len(clients))
	for _, c := range clients {
		clientMap[c.Provider()] = c
	}
	return &routingLLMService{
		selector:         NewSelector(cfg, hs),
		healthStore:      hs,
		clientByProvider: clientMap,
	}, nil
}

func (s *routingLLMService) Chat(ctx context.Context, req ChatRequest) (string, error) {
	targets := s.selector.SelectChatCandidates(req.Thinking)
	return ExecuteWithFallback(
		s.healthStore,
		CapabilityChat,
		targets,
		func(t *ModelTarget) ChatClient {
			return s.clientByProvider[Provider(t.Candidate.Provider)]
		},
		func(c ChatClient, t *ModelTarget) (string, error) {
			return c.Chat(ctx, req, t)
		},
	)
}

func (s *routingLLMService) StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback) error {
	targets := s.selector.SelectChatCandidates(req.Thinking)
	// 流式不能用泛型 ExecuteWithFallback —— 没法回放回调。
	// 简化：第一个 client 失败就调 cb.OnError 且返回错误；不做 Phase 7 的 ProbeBufferingCallback。
	if len(targets) == 0 {
		err := fmt.Errorf("no Chat candidates available")
		cb.OnError(err)
		return err
	}
	for i := range targets {
		target := &targets[i]
		client := s.clientByProvider[Provider(target.Candidate.Provider)]
		if client == nil {
			continue
		}
		if !s.healthStore.AllowCall(target.ID) {
			continue
		}
		err := client.StreamChat(ctx, req, cb, target)
		if err != nil {
			s.healthStore.MarkFailure(target.ID)
			// 流式 fallback 在 Phase 7 RAG Chat 用 ProbeBufferingCallback 时再做；
			// 这里第一个失败就直接报错，对齐 Phase 4 单候选的现有行为。
			cb.OnError(err)
			return err
		}
		s.healthStore.MarkSuccess(target.ID)
		return nil
	}
	err := fmt.Errorf("no available Chat client")
	cb.OnError(err)
	return err
}
