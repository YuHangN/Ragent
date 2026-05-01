package aiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YuHangN/ragent-go/config"
	"github.com/stretchr/testify/assert"
)

func makeEmbeddingTarget(serverURL string, dim int) *ModelTarget {
	return &ModelTarget{
		ID: "text-embedding-3-small",
		Candidate: config.ModelCandidate{
			ID: "text-embedding-3-small", Provider: "openai", Model: "text-embedding-3-small", Dimension: dim,
		},
		Provider: config.ProviderConfig{
			URL:       serverURL,
			APIKey:    "k",
			Endpoints: map[string]string{"embedding": "/v1/embeddings"},
		},
	}
}

func TestOpenAIEmbeddingClient_Embed_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float32{0.1, 0.2, 0.3}},
			},
		})
	}))
	defer server.Close()

	c := NewOpenAIEmbeddingClient()
	v, err := c.Embed(context.Background(), "hello", makeEmbeddingTarget(server.URL, 3))

	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, v)
}

func TestOpenAIEmbeddingClient_BatchOver32_SplitsRequests(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		input := req["input"].([]any)
		data := make([]map[string]any, len(input))
		for i := range input {
			data[i] = map[string]any{"index": i, "embedding": []float32{float32(i)}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()

	texts := make([]string, 70)
	for i := range texts {
		texts[i] = "x"
	}

	c := NewOpenAIEmbeddingClient()
	out, err := c.EmbedBatch(context.Background(), texts, makeEmbeddingTarget(server.URL, 1))
	assert.NoError(t, err)
	assert.Len(t, out, 70)
	assert.Equal(t, 3, requests)
}

func TestOpenAIEmbeddingClient_HTTP500_ReturnsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal"))
	}))
	defer server.Close()

	c := NewOpenAIEmbeddingClient()
	_, err := c.Embed(context.Background(), "hi", makeEmbeddingTarget(server.URL, 1))
	assert.Error(t, err)
	var ce *ClientError
	assert.ErrorAs(t, err, &ce)
	assert.Equal(t, ErrServerError, ce.Type)
	assert.Equal(t, "internal", ce.Message)
}
