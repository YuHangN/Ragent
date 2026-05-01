package aiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIChatClient 实现 ChatClient，覆盖 OpenAI / Ollama 等 OpenAI 兼容协议。
// 对齐 Java OpenAIStyleSseParser + 各 OpenAI-Style ChatClient 的合并版。
type OpenAIChatClient struct {
	provider Provider
	client   *http.Client
}

// NewOpenAIChatClient 默认绑定 ProviderOpenAI；想绑别的 provider 用 WithProvider。
func NewOpenAIChatClient() *OpenAIChatClient {
	return &OpenAIChatClient{
		provider: ProviderOpenAI,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

// WithProvider 让同一个 client 适配 OpenAI / Ollama 等共享 OpenAI 协议的 provider。
func (c *OpenAIChatClient) WithProvider(p Provider) *OpenAIChatClient {
	c.provider = p
	return c
}

func (c *OpenAIChatClient) Provider() Provider { return c.provider }

// ── request / response types ─────────────────────────────────────

type chatCompletionRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	Stream         bool          `json:"stream,omitempty"`
	Temperature    *float64      `json:"temperature,omitempty"`
	TopP           *float64      `json:"top_p,omitempty"`
	MaxTokens      *int          `json:"max_tokens,omitempty"`
	EnableThinking bool          `json:"enable_thinking,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *apiError `json:"error,omitempty"`
}

// ── sync chat ────────────────────────────────────────────────────

func (c *OpenAIChatClient) Chat(ctx context.Context, req ChatRequest, target *ModelTarget) (string, error) {
	url, err := ResolveURL(target.Provider, target.Candidate, CapabilityChat)
	if err != nil {
		return "", &ClientError{Type: ErrProviderError, Message: err.Error()}
	}
	body, _ := json.Marshal(c.buildReqBody(req, target.Candidate.Model, false))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+target.Provider.APIKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", NewHTTPError(resp.StatusCode, string(data))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", &ClientError{Type: ErrInvalidResponse, Message: err.Error(), Cause: err}
	}
	if result.Error != nil {
		return "", &ClientError{Type: ErrProviderError, Message: fmt.Sprintf("%s: %s", result.Error.Code, result.Error.Message)}
	}
	if len(result.Choices) == 0 {
		return "", &ClientError{Type: ErrInvalidResponse, Message: "empty choices"}
	}
	return result.Choices[0].Message.Content, nil
}

// ── stream chat ──────────────────────────────────────────────────

func (c *OpenAIChatClient) StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback, target *ModelTarget) error {
	url, err := ResolveURL(target.Provider, target.Candidate, CapabilityChat)
	if err != nil {
		return &ClientError{Type: ErrProviderError, Message: err.Error()}
	}
	body, _ := json.Marshal(c.buildReqBody(req, target.Candidate.Model, true))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+target.Provider.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return NewHTTPError(resp.StatusCode, string(data))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			cb.OnComplete()
			return nil
		}
		parseSseDelta(payload, cb)
	}
	if err := scanner.Err(); err != nil {
		return &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	cb.OnComplete()
	return nil
}

// sseDelta 一帧 stream chunk。
type sseDelta struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"` // OpenAI o1/兼容协议的思考链字段
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func parseSseDelta(payload string, cb StreamCallback) {
	var chunk sseDelta
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return // 静默丢弃畸形 chunk（Java 也是 log.warn + continue）
	}
	if len(chunk.Choices) == 0 {
		return
	}
	d := chunk.Choices[0].Delta
	if d.Content != "" {
		cb.OnContent(d.Content)
	}
	if d.ReasoningContent != "" {
		cb.OnThinking(d.ReasoningContent)
	}
}

func (c *OpenAIChatClient) buildReqBody(req ChatRequest, model string, stream bool) chatCompletionRequest {
	return chatCompletionRequest{
		Model:          model,
		Messages:       req.Messages,
		Stream:         stream,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		MaxTokens:      req.MaxTokens,
		EnableThinking: req.Thinking,
	}
}
