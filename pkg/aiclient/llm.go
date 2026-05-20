package aiclient

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

// LLMService 是业务侧使用的聊天模型入口。
//
// 接口保持稳定，具体实现负责在多个候选模型之间路由和 fallback。
type LLMService interface {
	Chat(ctx context.Context, req ChatRequest) (string, error)
	StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback) error
}

// routingLLMService 是带候选选择、熔断和 fallback 的 LLMService 实现。
type routingLLMService struct {
	selector         *Selector
	healthStore      *HealthStore
	clientByProvider map[Provider]ChatClient
}

// NewLLMService 构造路由式 LLMService。
//
// clients 中每个 ChatClient 的 Provider() 会作为路由表 key。
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

// Chat 选择可用聊天模型并执行非流式调用。
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

// StreamChat 选择可用聊天模型并执行流式调用。
func (s *routingLLMService) StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback) error {
	targets := s.selector.SelectChatCandidates(req.Thinking)
	// 流式输出一旦写入回调就无法完整回放，因此这里只在开始前选择目标；
	// 目标调用失败后直接通过回调报告错误，不尝试把半段输出切到下一个模型。
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
