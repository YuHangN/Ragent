package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenAIEmbeddingClient struct {
	provider Provider
	client   *http.Client
}

func NewOpenAIEmbeddingClient() *OpenAIEmbeddingClient {
	return &OpenAIEmbeddingClient{
		provider: ProviderOpenAI,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *OpenAIEmbeddingClient) WithProvider(p Provider) *OpenAIEmbeddingClient {
	c.provider = p
	return c
}

func (c *OpenAIEmbeddingClient) Provider() Provider { return c.provider }

func (c *OpenAIEmbeddingClient) Embed(ctx context.Context, text string, target *ModelTarget) ([]float32, error) {
	out, err := c.EmbedBatch(ctx, []string{text}, target)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, &ClientError{Type: ErrInvalidResponse, Message: "embedding returned 0 vectors"}
	}
	return out[0], nil
}

const embeddingBatchSize = 32

func (c *OpenAIEmbeddingClient) EmbedBatch(ctx context.Context, texts []string, target *ModelTarget) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float32, len(texts))
	for i := 0; i < len(texts); i += embeddingBatchSize {
		end := i + embeddingBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		part, err := c.doEmbed(ctx, texts[i:end], target)
		if err != nil {
			return nil, err
		}
		copy(out[i:], part)
	}
	return out, nil
}

type embeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
	Dimensions     int      `json:"dimensions,omitempty"` // OpenAI text-embedding-3-* 用，Ollama 等不识别会忽略
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *OpenAIEmbeddingClient) doEmbed(ctx context.Context, texts []string, target *ModelTarget) ([][]float32, error) {
	url, err := ResolveURL(target.Provider, target.Candidate, CapabilityEmbedding)
	if err != nil {
		return nil, &ClientError{Type: ErrProviderError, Message: err.Error()}
	}
	body, _ := json.Marshal(embeddingRequest{
		Model:          target.Candidate.Model,
		Input:          texts,
		EncodingFormat: "float",
		Dimensions:     target.Candidate.Dimension, // 0 时 omitempty 不发送
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+target.Provider.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, NewHTTPError(resp.StatusCode, string(data))
	}

	var result embeddingResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &ClientError{Type: ErrInvalidResponse, Message: err.Error(), Cause: err}
	}
	if result.Error != nil {
		return nil, &ClientError{Type: ErrProviderError, Message: fmt.Sprintf("%s: %s", result.Error.Code, result.Error.Message)}
	}
	if len(result.Data) != len(texts) {
		return nil, &ClientError{
			Type:    ErrInvalidResponse,
			Message: fmt.Sprintf("expected %d vectors, got %d", len(texts), len(result.Data)),
		}
	}
	vectors := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(texts) {
			vectors[d.Index] = d.Embedding
		}
	}
	return vectors, nil
}
