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

// OpenAIChatClient 实现 OpenAI 兼容的聊天补全协议。
//
// 该实现可通过 WithProvider 复用到 OpenAI、Ollama 等共享 chat completions
// 请求格式和 SSE 响应格式的 provider。
type OpenAIChatClient struct {
	provider Provider
	client   *http.Client
}

// NewOpenAIChatClient 构造默认绑定 ProviderOpenAI 的聊天客户端。
func NewOpenAIChatClient() *OpenAIChatClient {
	return &OpenAIChatClient{
		provider: ProviderOpenAI,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

// WithProvider 将客户端绑定到指定 provider，便于复用 OpenAI 兼容协议实现。
func (c *OpenAIChatClient) WithProvider(p Provider) *OpenAIChatClient {
	c.provider = p
	return c
}

// Provider 返回当前客户端负责的 provider。
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

// Chat 调用 OpenAI 兼容的非流式聊天补全接口。
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

// StreamChat 调用 OpenAI 兼容的流式聊天补全接口。
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

// sseDelta 表示 OpenAI 兼容 SSE 流中的一帧增量响应。
type sseDelta struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"` // 部分兼容协议用于承载思考内容。
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func parseSseDelta(payload string, cb StreamCallback) {
	var chunk sseDelta
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return // 流式输出中跳过畸形帧，避免单帧解析失败中断整段回答。
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
