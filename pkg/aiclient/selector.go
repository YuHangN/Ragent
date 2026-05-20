package aiclient

import (
	"sort"

	"github.com/YuHangN/ragent-go/config"
)

// Selector 按配置、默认模型和健康状态选择可调用模型候选。
type Selector struct {
	cfg         *config.AIConfig
	healthStore *HealthStore
}

// NewSelector 构造模型候选选择器。
func NewSelector(cfg *config.AIConfig, hs *HealthStore) *Selector {
	return &Selector{cfg: cfg, healthStore: hs}
}

// SelectChatCandidates 返回聊天能力的候选目标列表。
//
// thinking 为 true 时，只保留支持思考模式的候选，并优先提升 DeepThinkingModel。
func (s *Selector) SelectChatCandidates(thinking bool) []ModelTarget {
	cfg := s.cfg.Chat
	firstChoice := cfg.DefaultModel
	if thinking && cfg.DeepThinkingModel != "" {
		firstChoice = cfg.DeepThinkingModel
	}
	return s.selectCandidates(cfg.Candidates, firstChoice, thinking)
}

// SelectEmbeddingCandidates 返回向量化能力的候选目标列表。
func (s *Selector) SelectEmbeddingCandidates() []ModelTarget {
	cfg := s.cfg.Embedding
	return s.selectCandidates(cfg.Candidates, cfg.DefaultModel, false)
}

// SelectRerankCandidates 返回重排能力的候选目标列表。
func (s *Selector) SelectRerankCandidates() []ModelTarget {
	cfg := s.cfg.Rerank
	return s.selectCandidates(cfg.Candidates, cfg.DefaultModel, false)
}

// SelectDefaultEmbedding 返回首个可用向量化候选；没有可用候选时返回 nil。
func (s *Selector) SelectDefaultEmbedding() *ModelTarget {
	targets := s.SelectEmbeddingCandidates()
	if len(targets) == 0 {
		return nil
	}
	return &targets[0]
}

func (s *Selector) selectCandidates(candidates []config.ModelCandidate, firstChoice string, thinking bool) []ModelTarget {
	// 先过滤禁用项和不满足 thinking 要求的候选。
	filtered := make([]config.ModelCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !c.IsEnabled() {
			continue
		}
		if thinking && !c.SupportsThinking {
			continue
		}
		filtered = append(filtered, c)
	}

	// priority 越小优先级越高；同优先级下按 ID 保持稳定顺序。
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority < filtered[j].Priority
		}
		return filtered[i].ID < filtered[j].ID
	})

	// 默认模型或深度思考模型命中时提升到最前，覆盖普通 priority 排序。
	if firstChoice != "" {
		for i, c := range filtered {
			if c.ID == firstChoice {
				if i != 0 {
					promoted := filtered[i]
					copy(filtered[1:i+1], filtered[0:i])
					filtered[0] = promoted
				}
				break
			}
		}
	}

	// 转为可调用目标，并跳过熔断中或 provider 缺失的候选。
	result := make([]ModelTarget, 0, len(filtered))
	for _, c := range filtered {
		target := NewModelTarget(c, config.ProviderConfig{})
		if s.healthStore.IsOpen(target.ID) {
			continue
		}
		// NOOP 是合法的特殊 provider，即使不在 providers 配置中也允许。
		provider, ok := s.cfg.Providers[c.Provider]
		if !ok && !ProviderNoop.Matches(c.Provider) {
			continue
		}
		target.Provider = provider
		result = append(result, target)
	}
	return result
}
