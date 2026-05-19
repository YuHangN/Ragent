package retrieval

import (
	"context"
	"testing"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLLM 测试用 LLMService 桩
type stubLLM struct {
	resp string
	err  error
}

func (s *stubLLM) Chat(_ context.Context, _ aiclient.ChatRequest) (string, error) {
	return s.resp, s.err
}
func (s *stubLLM) StreamChat(_ context.Context, _ aiclient.ChatRequest, _ aiclient.StreamCallback) error {
	return nil
}

func TestQueryRewrite_Disabled(t *testing.T) {
	svc := NewQueryRewriteService(nil, config.QueryRewriteConfig{Enabled: false})
	result, err := svc.Rewrite(context.Background(), "什么是 RAG？", nil)
	require.NoError(t, err)
	assert.Equal(t, "什么是 RAG？", result.RewrittenQuery)
	assert.Equal(t, []string{"什么是 RAG？"}, result.SubQuestions)
}

func TestQueryRewrite_LLMSuccess(t *testing.T) {
	llm := &stubLLM{
		resp: `{"rewritten":"RAG 系统的工作原理","sub_questions":["RAG 是什么","RAG 如何检索文档"]}`,
	}
	svc := NewQueryRewriteService(llm, config.QueryRewriteConfig{
		Enabled:            true,
		MaxHistoryMessages: 6,
		MaxHistoryChars:    2000,
	})

	result, err := svc.Rewrite(context.Background(), "RAG 怎么用？", nil)
	require.NoError(t, err)
	assert.Equal(t, "RAG 系统的工作原理", result.RewrittenQuery)
	assert.Len(t, result.SubQuestions, 2)
}

func TestQueryRewrite_LLMFallback_NonJSON(t *testing.T) {
	llm := &stubLLM{resp: "抱歉，我无法改写"}
	svc := NewQueryRewriteService(llm, config.QueryRewriteConfig{Enabled: true})
	result, err := svc.Rewrite(context.Background(), "原始问题", nil)
	require.NoError(t, err) // 改写失败降级，不报错
	assert.Equal(t, "原始问题", result.RewrittenQuery)
}

func TestQueryRewrite_MarkdownCodeFence(t *testing.T) {
	// 验证 LLM 把 JSON 包在 ```json ... ``` 里也能解析
	llm := &stubLLM{
		resp: "```json\n{\"rewritten\":\"X\",\"sub_questions\":[\"Y\"]}\n```",
	}
	svc := NewQueryRewriteService(llm, config.QueryRewriteConfig{Enabled: true})
	result, err := svc.Rewrite(context.Background(), "Q", nil)
	require.NoError(t, err)
	assert.Equal(t, "X", result.RewrittenQuery)
}

func TestTrimHistory(t *testing.T) {
	history := []aiclient.ChatMessage{
		aiclient.User("问题1"),
		aiclient.Assistant("回答1"),
		aiclient.User("问题2"),
		aiclient.Assistant("回答2"),
	}
	trimmed := trimHistory(history, 2, 10000)
	assert.Len(t, trimmed, 2)
	assert.Equal(t, "问题2", trimmed[0].Content)
}
