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

	"github.com/YuHangN/ragent-go/config"
)

type LLMService interface {
	Chat(ctx context.Context, req ChatRequest) (string, error)
	StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback) error
}

type httpLLMService struct {
	apiURL string
	apiKey string
	model  string
	client *http.Client
}

func NewLLMService(cfg *config.AIConfig) (LLMService, error) {
	candidate, provider, err := resolveDefault(
		cfg.Chat.DefaultModel,
		cfg.Chat.Candidates,
		cfg.Providers,
	)
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	apiURL := resolveEndpoint(provider, "chat", "/v1/chat/completions")
	return &httpLLMService{
		apiURL: apiURL,
		apiKey: provider.APIKey,
		model:  candidate.Model,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

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

func (s *httpLLMService) Chat(ctx context.Context, req ChatRequest) (string, error) {
	// 构造请求体
	body, _ := json.Marshal(s.buildReqBody(req, false))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	// 发送请求
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm HTTP: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, data)
	}

	// 解析响应
	var result chatCompletionResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("llm parse: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("llm API error %s: %s", result.Error.Code, result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm: empty choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// ── stream chat ──────────────────────────────────────────────────

func (s *httpLLMService) StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback) error {
	body, _ := json.Marshal(s.buildReqBody(req, true))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("llm stream build: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("llm stream HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm stream HTTP %d: %s", resp.StatusCode, data)
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
		return fmt.Errorf("llm stream read: %w", err)
	}
	cb.OnComplete()
	return nil
}

// sseDelta is one streaming chunk from /v1/chat/completions with stream=true.
type sseDelta struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"` // DeepSeek / thinking models
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func parseSseDelta(payload string, cb StreamCallback) {
	var chunk sseDelta
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return // malformed chunk — skip silently (Java also uses log.warn + continue)
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

func (s *httpLLMService) buildReqBody(req ChatRequest, stream bool) chatCompletionRequest {
	return chatCompletionRequest{
		Model:          s.model,
		Messages:       req.Messages,
		Stream:         stream,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		MaxTokens:      req.MaxTokens,
		EnableThinking: req.Thinking,
	}
}
