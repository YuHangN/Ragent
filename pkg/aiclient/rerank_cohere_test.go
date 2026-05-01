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

func makeRerankTarget(serverURL string) *ModelTarget {
	return &ModelTarget{
		ID: "rerank-multilingual-v3",
		Candidate: config.ModelCandidate{
			ID: "rerank-multilingual-v3", Provider: "cohere", Model: "rerank-multilingual-v3.0",
		},
		Provider: config.ProviderConfig{
			URL:       serverURL,
			APIKey:    "k",
			Endpoints: map[string]string{"rerank": "/v1/rerank"},
		},
	}
}

func TestNoopRerankClient_TruncatesToTopN(t *testing.T) {
	c := NewNoopRerankClient()
	chunks := []RetrievedChunk{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	out, err := c.Rerank(context.Background(), "q", chunks, 2, nil)
	assert.NoError(t, err)
	assert.Len(t, out, 2)
}

func TestCohereRerankClient_ReordersByScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"index": 2, "relevance_score": 0.9},
				{"index": 0, "relevance_score": 0.5},
			},
		})
	}))
	defer server.Close()

	chunks := []RetrievedChunk{
		{ID: "a", Text: "doc-a"},
		{ID: "b", Text: "doc-b"},
		{ID: "c", Text: "doc-c"},
	}
	c := NewCohereRerankClient()
	out, err := c.Rerank(context.Background(), "q", chunks, 2, makeRerankTarget(server.URL))
	assert.NoError(t, err)
	assert.Len(t, out, 2)
	assert.Equal(t, "c", out[0].ID)
	assert.InDelta(t, 0.9, out[0].Score, 0.001)
}

func TestCohereRerankClient_SkipsHTTPWhenTopNAlreadySatisfied(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	chunks := []RetrievedChunk{{ID: "a"}, {ID: "b"}}
	c := NewCohereRerankClient()
	out, err := c.Rerank(context.Background(), "q", chunks, 5, makeRerankTarget(server.URL))
	assert.NoError(t, err)
	assert.Len(t, out, 2)
	assert.False(t, called, "len(candidates) <= topN 时应跳过 HTTP")
}
