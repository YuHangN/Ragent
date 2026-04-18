package aiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YuHangN/ragent-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLLMService(serverURL string) LLMService {
	cfg := &config.AIConfig{
		Providers: map[string]config.ProviderConfig{
			"mock": {URL: serverURL, APIKey: "test-key"},
		},
		Chat: config.ChatModelConfig{
			DefaultModel: "test-chat",
			Candidates: []config.ModelCandidate{
				{ID: "test-chat", Provider: "mock", Model: "mock-llm"},
			},
		},
	}

	svc, _ := NewLLMService(cfg)
	return svc
}

func TestChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "hello world"}, "finish_reason": "stop"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	svc := newTestLLMService(srv.URL)
	reply, err := svc.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{User("hi")},
	})

	require.NoError(t, err)
	assert.Equal(t, "hello world", reply)
}

func TestChat_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	svc := newTestLLMService(srv.URL)
	_, err := svc.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{User("hi")},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

// collectingCallback collects streamed output for assertion.
type collectingCallback struct {
	contents  []string
	thinkings []string
	completed bool
	err       error
}

func (c *collectingCallback) OnContent(s string)  { c.contents = append(c.contents, s) }
func (c *collectingCallback) OnThinking(s string) { c.thinkings = append(c.thinkings, s) }
func (c *collectingCallback) OnComplete()         { c.completed = true }
func (c *collectingCallback) OnError(e error)     { c.err = e }

func TestStreamChat_ContentDelivered(t *testing.T) {
	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n" +
		"data: [DONE]\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseBody))
	}))
	defer srv.Close()

	svc := newTestLLMService(srv.URL)
	cb := &collectingCallback{}
	err := svc.StreamChat(context.Background(), ChatRequest{
		Messages: []ChatMessage{User("hi")},
	}, cb)

	require.NoError(t, err)
	assert.Equal(t, []string{"hel", "lo"}, cb.contents)
	assert.True(t, cb.completed)
}

func TestStreamChat_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	svc := newTestLLMService(srv.URL)
	cb := &collectingCallback{}
	err := svc.StreamChat(context.Background(), ChatRequest{
		Messages: []ChatMessage{User("hi")},
	}, cb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}
