package aiclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YuHangN/ragent-go/config"
	"github.com/stretchr/testify/assert"
)

type stubChatClient struct {
	provider Provider
	chatResp string
	chatErr  error
	calls    int
}

func (s *stubChatClient) Provider() Provider { return s.provider }

func (s *stubChatClient) Chat(_ context.Context, _ ChatRequest, _ *ModelTarget) (string, error) {
	s.calls++
	return s.chatResp, s.chatErr
}

func (s *stubChatClient) StreamChat(_ context.Context, _ ChatRequest, _ StreamCallback, _ *ModelTarget) error {
	s.calls++
	return s.chatErr
}

func makeMultiCandidateConfig() *config.AIConfig {
	return &config.AIConfig{
		Providers: map[string]config.ProviderConfig{
			"openai": {URL: "https://api.openai.com", APIKey: "k1", Endpoints: map[string]string{"chat": "/v1/chat/completions"}},
			"ollama": {URL: "http://localhost:11434", APIKey: "", Endpoints: map[string]string{"chat": "/v1/chat/completions"}},
		},
		Chat: config.ChatModelConfig{
			DefaultModel: "primary",
			Candidates: []config.ModelCandidate{
				{ID: "primary", Provider: "openai", Model: "gpt-4o-mini", Priority: 1},
				{ID: "fallback", Provider: "ollama", Model: "llama3.1:8b", Priority: 2},
			},
		},
	}
}

func TestRoutingLLMService_Chat_PrimarySucceeds(t *testing.T) {
	cfg := makeMultiCandidateConfig()
	hs := NewHealthStore(3, time.Second)
	primary := &stubChatClient{provider: ProviderOpenAI, chatResp: "ok-primary"}
	fallback := &stubChatClient{provider: ProviderOllama, chatResp: "ok-fallback"}
	svc, err := NewLLMService(cfg, hs, []ChatClient{primary, fallback})
	assert.NoError(t, err)

	resp, err := svc.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{User("hi")}})
	assert.NoError(t, err)
	assert.Equal(t, "ok-primary", resp)
	assert.Equal(t, 1, primary.calls)
	assert.Equal(t, 0, fallback.calls)
}

func TestRoutingLLMService_Chat_FallsBackOnError(t *testing.T) {
	cfg := makeMultiCandidateConfig()
	hs := NewHealthStore(3, time.Second)
	primary := &stubChatClient{provider: ProviderOpenAI, chatErr: errors.New("primary down")}
	fallback := &stubChatClient{provider: ProviderOllama, chatResp: "ok-fallback"}
	svc, _ := NewLLMService(cfg, hs, []ChatClient{primary, fallback})

	resp, err := svc.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{User("hi")}})
	assert.NoError(t, err)
	assert.Equal(t, "ok-fallback", resp)
	assert.Equal(t, 1, primary.calls)
	assert.Equal(t, 1, fallback.calls)
}

func TestRoutingLLMService_Chat_AllFail(t *testing.T) {
	cfg := makeMultiCandidateConfig()
	hs := NewHealthStore(3, time.Second)
	primary := &stubChatClient{provider: ProviderOpenAI, chatErr: errors.New("primary down")}
	fallback := &stubChatClient{provider: ProviderOllama, chatErr: errors.New("fallback down")}
	svc, _ := NewLLMService(cfg, hs, []ChatClient{primary, fallback})

	_, err := svc.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{User("hi")}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all Chat candidates failed")
}
