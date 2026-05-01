package aiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YuHangN/ragent-go/config"
	"github.com/stretchr/testify/assert"
)

func makeChatTarget(serverURL string) *ModelTarget {
	return &ModelTarget{
		ID: "test-model",
		Candidate: config.ModelCandidate{
			ID: "test-model", Provider: "openai", Model: "gpt-4o-mini",
		},
		Provider: config.ProviderConfig{
			URL:    serverURL,
			APIKey: "test-key",
			Endpoints: map[string]string{
				"chat": "/v1/chat/completions",
			},
		},
	}
}

func TestOpenAIChatClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "hello"}},
			},
		})
	}))
	defer server.Close()

	c := NewOpenAIChatClient()
	resp, err := c.Chat(context.Background(),
		ChatRequest{Messages: []ChatMessage{User("hi")}},
		makeChatTarget(server.URL))
	assert.NoError(t, err)
	assert.Equal(t, "hello", resp)
}

func TestOpenAIChatClient_Chat_HTTP429_ReturnsRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limit","message":"slow down"}}`))
	}))
	defer server.Close()

	c := NewOpenAIChatClient()
	_, err := c.Chat(context.Background(),
		ChatRequest{Messages: []ChatMessage{User("hi")}},
		makeChatTarget(server.URL))
	assert.Error(t, err)
	var ce *ClientError
	assert.ErrorAs(t, err, &ce)
	assert.Equal(t, ErrRateLimited, ce.Type)
}

type captureCallback struct {
	content  strings.Builder
	thinking strings.Builder
	done     bool
	errored  error
}

func (c *captureCallback) OnContent(d string)  { c.content.WriteString(d) }
func (c *captureCallback) OnThinking(d string) { c.thinking.WriteString(d) }
func (c *captureCallback) OnComplete()         { c.done = true }
func (c *captureCallback) OnError(e error)     { c.errored = e }

func TestOpenAIChatClient_StreamChat_AccumulatesContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	cb := &captureCallback{}
	c := NewOpenAIChatClient()
	err := c.StreamChat(context.Background(),
		ChatRequest{Messages: []ChatMessage{User("hi")}},
		cb,
		makeChatTarget(server.URL))
	assert.NoError(t, err)
	assert.Equal(t, "hello world", cb.content.String())
	assert.True(t, cb.done)
}
