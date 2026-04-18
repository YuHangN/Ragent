package rag

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

const rewritePromptTemplate = `你是一个查询改写助手。请将用户问题改写为更精确、更适合向量检索的表达，并根据问题复杂度拆分为若干个独立的子问题（简单问题只需 1 个子问题即可）。

用户问题：{{question}}

请以 JSON 格式回复，不要输出任何其他内容：
{"rewritten": "改写后的问题", "sub_questions": ["子问题1", "子问题2"]}`

// QueryRewriteService 对用户原始问题执行 LLM 改写和子问题拆分。
type QueryRewriteService struct {
	llm    aiclient.LLMService
	config config.QueryRewriteConfig
}

func NewQueryRewriteService(llm aiclient.LLMService, cfg config.QueryRewriteConfig) *QueryRewriteService {
	return &QueryRewriteService{llm: llm, config: cfg}
}

// Rewrite 改写问题并拆分子问题。若配置禁用改写，直接返回原问题。
func (s *QueryRewriteService) Rewrite(ctx context.Context, question string, history []aiclient.ChatMessage) (RewriteResult, error) {
	// 如果改写功能未启用，直接返回原问题作为改写结果和唯一子问题。
	fallback := RewriteResult{
		RewrittenQuery: question,
		SubQuestions:   []string{question},
	}

	if !s.config.Enabled {
		return fallback, nil
	}

	// 截取历史消息，避免 token 过长
	history = trimHistory(history, s.config.MaxHistoryMessages, s.config.MaxHistoryChars)
	prompt := strings.ReplaceAll(rewritePromptTemplate, "{{question}}", question)
	messages := append(history, aiclient.User(prompt))
	resp, err := s.llm.Chat(ctx, aiclient.ChatRequest{Messages: messages})
	if err != nil {
		// 改写失败时降级为原问题，不中断链路
		return fallback, nil
	}

	var result struct {
		Rewritten    string   `json:"rewritten"`
		SubQuestions []string `json:"sub_questions"`
	}

	// LLM 有时会在 JSON 外包裹 markdown code block，先清理
	cleaned := strings.TrimSpace(resp)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")

	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return fallback, nil
	}

	if result.Rewritten == "" {
		result.Rewritten = question
	}
	if len(result.SubQuestions) == 0 {
		result.SubQuestions = []string{result.Rewritten}
	}

	return RewriteResult{
		RewrittenQuery: result.Rewritten,
		SubQuestions:   result.SubQuestions,
	}, nil
}

// trimHistory 截取最近 maxMessages 条消息，并限制总字符数。
func trimHistory(history []aiclient.ChatMessage, maxMessages, maxChars int) []aiclient.ChatMessage {
	if len(history) == 0 {
		return nil
	}
	if maxMessages > 0 && len(history) > maxMessages {
		history = history[len(history)-maxMessages:]
	}
	if maxChars <= 0 {
		return history
	}
	total := 0
	for i := len(history) - 1; i >= 0; i-- {
		total += len(history[i].Content)
		if total > maxChars {
			return history[i+1:]
		}
	}
	return history
}
