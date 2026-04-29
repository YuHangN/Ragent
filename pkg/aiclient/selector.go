package aiclient

import (
	"sort"

	"github.com/YuHangN/ragent-go/config"
)

type Selector struct {
	cfg         *config.AIConfig
	healthStore *HealthStore
}

func NewSelector(cfg *config.AIConfig, hs *HealthStore) *Selector {
	return &Selector{cfg: cfg, healthStore: hs}
}

func (s *Selector) SelectChatCandidates(thinking bool) []ModelTarget {
	cfg := s.cfg.Chat
	firstChoice := cfg.DefaultModel
	if thinking && cfg.DeepThinkingModel != "" {
		firstChoice = cfg.DeepThinkingModel
	}
	return s.selectCandidates(cfg.Candidates, firstChoice, thinking)
}

// SelectEmbeddingCandidates 返回 embedding 候选列表。
func (s *Selector) SelectEmbeddingCandidates() []ModelTarget {
	cfg := s.cfg.Embedding
	return s.selectCandidates(cfg.Candidates, cfg.DefaultModel, false)
}

// SelectRerankCandidates 返回 rerank 候选列表。
func (s *Selector) SelectRerankCandidates() []ModelTarget {
	cfg := s.cfg.Rerank
	return s.selectCandidates(cfg.Candidates, cfg.DefaultModel, false)
}

// SelectDefaultEmbedding 返回首个可用 embedding 候选，没有则返回 nil。
func (s *Selector) SelectDefaultEmbedding() *ModelTarget {
	targets := s.SelectEmbeddingCandidates()
	if len(targets) == 0 {
		return nil
	}
	return &targets[0]
}

func (s *Selector) selectCandidates(candidates []config.ModelCandidate, firstChoice string, thinking bool) []ModelTarget {
	// 1. 过滤：未禁用 + 满足 thinking 要求
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

	// 2. 排序：priority asc，同 priority 按 ID 字典序
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority < filtered[j].Priority
		}
		return filtered[i].ID < filtered[j].ID
	})

	// 3. 把 firstChoice 提到最前
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

	// 4. 转 ModelTarget：跳过熔断 / 跳过 provider 缺失
	result := make([]ModelTarget, 0, len(filtered))
	for _, c := range filtered {
		target := NewModelTarget(c, config.ProviderConfig{})
		if s.healthStore.IsOpen(target.ID) {
			continue
		}
		// 检查 provider 是否存在；NOOP 是合法的特殊 provider
		provider, ok := s.cfg.Providers[c.Provider]
		if !ok && !ProviderNoop.Matches(c.Provider) {
			continue
		}
		target.Provider = provider
		result = append(result, target)
	}
	return result
}
