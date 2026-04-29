package aiclient

import (
	"testing"
	"time"

	"github.com/YuHangN/ragent-go/config"
	"github.com/stretchr/testify/assert"
)

func makeAIConfig() *config.AIConfig {
	return &config.AIConfig{
		Providers: map[string]config.ProviderConfig{
			"openai": {URL: "https://api.openai.com"},
			"ollama": {URL: "http://localhost:11434"},
		},
		Chat: config.ChatModelConfig{
			DefaultModel:      "gpt-4o-mini",
			DeepThinkingModel: "gpt-o1-mini",
			Candidates: []config.ModelCandidate{
				{ID: "gpt-4o-mini", Provider: "openai", Model: "gpt-4o-mini", Priority: 2},
				{ID: "gpt-o1-mini", Provider: "openai", Model: "o1-mini", Priority: 1, SupportsThinking: true},
				{ID: "ollama-local", Provider: "ollama", Model: "llama3.1:8b", Priority: 5},
			},
		},
	}
}

func TestSelector_Chat_NormalMode_DefaultModelPromoted(t *testing.T) {
	cfg := makeAIConfig()
	hs := NewHealthStore(3, time.Second)
	s := NewSelector(cfg, hs)

	targets := s.SelectChatCandidates(false)
	if assert.NotEmpty(t, targets) {
		assert.Equal(t, "gpt-4o-mini", targets[0].ID, "default 应该被提到最前")
		assert.Equal(t, "ollama-local", targets[2].ID, "最后应该是优先级最低的")
	}
	assert.Len(t, targets, 3)
}

func TestSelector_Chat_ThinkingMode_FiltersAndPromotesDeep(t *testing.T) {
	cfg := makeAIConfig()
	hs := NewHealthStore(3, time.Second)
	s := NewSelector(cfg, hs)

	targets := s.SelectChatCandidates(true)
	if assert.NotEmpty(t, targets) {
		assert.Equal(t, "gpt-o1-mini", targets[0].ID)
	}
	for _, tt := range targets {
		assert.True(t, tt.Candidate.SupportsThinking, "thinking 模式应过滤掉不支持的")
	}
	assert.Len(t, targets, 1, "只有一个候选支持 thinking")
}

func TestSelector_Chat_PriorityOrder(t *testing.T) {
	cfg := makeAIConfig()
	cfg.Chat.DefaultModel = ""
	hs := NewHealthStore(3, time.Second)
	s := NewSelector(cfg, hs)

	targets := s.SelectChatCandidates(false)
	assert.Equal(t, "gpt-o1-mini", targets[0].ID)  // priority 1
	assert.Equal(t, "gpt-4o-mini", targets[1].ID)  // priority 2
	assert.Equal(t, "ollama-local", targets[2].ID) // priority 5
}

func TestSelector_FilterDisabled(t *testing.T) {
	cfg := makeAIConfig()
	disabled := false
	cfg.Chat.Candidates[0].Enabled = &disabled
	hs := NewHealthStore(3, time.Second)
	s := NewSelector(cfg, hs)

	targets := s.SelectChatCandidates(false)
	for _, tt := range targets {
		assert.NotEqual(t, "gpt-4o-mini", tt.ID)
	}
	assert.Equal(t, "gpt-o1-mini", targets[0].ID)
	assert.Equal(t, "ollama-local", targets[1].ID)
	assert.Len(t, targets, 2, "被禁用的模型不应该出现在候选列表中")
}

func TestSelector_FilterCircuitOpen(t *testing.T) {
	cfg := makeAIConfig()
	hs := NewHealthStore(1, 1*time.Hour) // 阈值 1，1 次失败即熔断
	hs.MarkFailure("gpt-4o-mini")
	s := NewSelector(cfg, hs)

	targets := s.SelectChatCandidates(false)
	for _, tt := range targets {
		assert.NotEqual(t, "gpt-4o-mini", tt.ID, "熔断中的应被过滤")
	}
}

func TestSelector_FilterMissingProvider(t *testing.T) {
	cfg := makeAIConfig()
	delete(cfg.Providers, "ollama") // 删掉 provider，候选应被跳过
	hs := NewHealthStore(3, time.Second)
	s := NewSelector(cfg, hs)

	targets := s.SelectChatCandidates(false)
	for _, tt := range targets {
		assert.NotEqual(t, "ollama-local", tt.ID)
	}
}
