package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/YuHangN/ragent-go/config"
)

// EmbeddingService converts text to dense float vectors.
type EmbeddingService interface {
	// Embed embeds a single text. Internally calls EmbedBatch.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch embeds a batch of texts. Returns a slice of float32 vectors.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the vector size for schema validation (e.g. 1024, 1536).
	Dimension() int
}

// httpEmbeddingService calls OpenAI-compatible /v1/embeddings.
type httpEmbeddingService struct {
	apiURL    string
	apiKey    string
	model     string
	dimension int
	client    *http.Client
}

// NewEmbeddingService builds an EmbeddingService from AIConfig.
func NewEmbeddingService(cfg *config.AIConfig) (EmbeddingService, error) {
	candidate, provider, err := resolveDefault(
		cfg.Embedding.DefaultModel,
		cfg.Embedding.Candidates,
		cfg.Providers,
	)
	if err != nil {
		return nil, fmt.Errorf("embedding: %w", err)
	}

	apiURL := resolveEndpoint(provider, "embedding", "/v1/embeddings")
	return &httpEmbeddingService{
		apiURL:    apiURL,
		apiKey:    provider.APIKey,
		model:     candidate.Model,
		dimension: candidate.Dimension,
		client:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *httpEmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := s.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	return results[0], nil
}

const embeddingBatchSize = 32

func (s *httpEmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	out := make([][]float32, len(texts))
	for i := 0; i < len(texts); i += embeddingBatchSize {
		end := i + embeddingBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		part, err := s.doEmbed(ctx, texts[i:end])
		if err != nil {
			return nil, err
		}
		copy(out[i:], part)
	}

	return out, nil
}

func (s *httpEmbeddingService) Dimension() int { return s.dimension }

// ── request / response structs ────────────────────────────────────

type embeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *apiError `json:"error,omitempty"`
}

// apiError covers the {"error":{"code":"...","message":"..."}} shape
// returned by most OpenAI-compatible providers on 4xx.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *httpEmbeddingService) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	// 1. 拼接请求体，包含模型名、文本列表、编码格式等信息。
	body, _ := json.Marshal(embeddingRequest{
		Model:          s.model,
		Input:          texts,
		EncodingFormat: "float",
	})
	// 2. 创建 HTTP POST 请求，设置必要的 header（Content-Type、Authorization）
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	// 3. 发送请求，读取响应体，检查 HTTP 状态码。
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding HTTP: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding HTTP %d: %s", resp.StatusCode, data)
	}

	var result embeddingResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("embedding parse: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("embedding API error %s: %s", result.Error.Code, result.Error.Message)
	}
	if len(result.Data) != len(texts) {
		return nil, fmt.Errorf("embedding: expected %d vectors, got %d", len(texts), len(result.Data))
	}

	// API may return objects in any order — place each vector by its index field.
	vectors := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(texts) {
			vectors[d.Index] = d.Embedding
		}
	}

	return vectors, nil
}

// ── shared helpers (used by embedding, llm, rerank) ───────────────

// resolveDefault finds the ModelCandidate whose ID matches defaultModel
func resolveDefault(
	defaultModel string,
	candidates []config.ModelCandidate,
	providers map[string]config.ProviderConfig,
) (config.ModelCandidate, config.ProviderConfig, error) {
	for _, c := range candidates {
		// 如果没有指定 defaultModel，就选第一个；否则选 ID 匹配的那个。
		if defaultModel == "" || c.ID == defaultModel {
			p, ok := providers[c.Provider]
			if !ok {
				return config.ModelCandidate{}, config.ProviderConfig{},
					fmt.Errorf("provider %q not found in providers map", c.Provider)
			}
			return c, p, nil
		}
	}

	return config.ModelCandidate{}, config.ProviderConfig{},
		fmt.Errorf("no candidate found for model %q", defaultModel)
}

// resolveEndpoint returns provider.URL + endpoints[key] when the endpoints map
// has an entry for key; otherwise returns provider.URL + fallback.
func resolveEndpoint(p config.ProviderConfig, key, fallback string) string {
	if ep, ok := p.Endpoints[key]; ok {
		return p.URL + ep
	}

	return p.URL + fallback
}
