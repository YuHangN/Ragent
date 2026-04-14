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

// newTestEmbeddingService points the service at a mock HTTP server URL.
func newTestEmbeddingService(serverURL string) EmbeddingService {
	cfg := &config.AIConfig{
		Providers: map[string]config.ProviderConfig{
			"mock": {URL: serverURL, APIKey: "test-key"},
		},
		Embedding: config.EmbeddingModelConfig{
			DefaultModel: "test-model",
			Candidates: []config.ModelCandidate{
				{ID: "test-model", Provider: "mock", Model: "mock-embed", Dimension: 4},
			},
		},
	}
	svc, _ := NewEmbeddingService(cfg)
	return svc
}

func TestEmbed_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float32{0.1, 0.2, 0.3, 0.4}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	svc := newTestEmbeddingService(srv.URL)
	vec, err := svc.Embed(context.Background(), "hello")
	require.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3, 0.4}, vec)
}

func TestEmbedBatch_Empty(t *testing.T) {
	svc := newTestEmbeddingService("http://unused")
	out, err := svc.EmbedBatch(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestEmbedBatch_CountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// returns 0 vectors for 1 input → mismatch
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer srv.Close()

	svc := newTestEmbeddingService(srv.URL)
	_, err := svc.EmbedBatch(context.Background(), []string{"text"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 1 vectors, got 0")
}

func TestEmbed_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := newTestEmbeddingService(srv.URL)
	_, err := svc.Embed(context.Background(), "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestEmbeddingService_Dimension(t *testing.T) {
	svc := newTestEmbeddingService("http://unused")
	assert.Equal(t, 4, svc.Dimension())
}
