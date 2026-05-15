package ingestion

import (
	"context"
	"errors"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLLM 是 ingestion 测试用的 aiclient.LLMService 桩。
//
// onChat / onStream 由用例注入；StreamChat 在 Enhancer / Enricher 测试中不会被
// 调用（两个节点都用 Chat），保留只是为了实现接口。
type stubLLM struct {
	onChat   func(req aiclient.ChatRequest) (string, error)
	onStream func(req aiclient.ChatRequest, cb aiclient.StreamCallback) error
}

func (s *stubLLM) Chat(_ context.Context, req aiclient.ChatRequest) (string, error) {
	return s.onChat(req)
}
func (s *stubLLM) StreamChat(_ context.Context, req aiclient.ChatRequest, cb aiclient.StreamCallback) error {
	if s.onStream == nil {
		return nil
	}
	return s.onStream(req, cb)
}

// TestEnhancer_HappyPath 验证 LLM 返回合法 JSON 时，关键字段被正确写到 ic。
func TestEnhancer_HappyPath(t *testing.T) {
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return `{"summary":"这是文档摘要","keywords":["RAG","检索","增强"]}`, nil
	}}
	node := NewEnhancerNode(llm)
	ic := &IngestionContext{RawText: "原始全文..."}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success)
	require.True(t, res.ShouldContinue)
	assert.Equal(t, []string{"RAG", "检索", "增强"}, ic.Keywords)
	assert.Equal(t, "这是文档摘要", ic.Metadata["doc_summary"])
}

// TestEnhancer_LLMFailure_ReturnsOKWithoutPolluting 验证 LLM 调用失败时
// 节点返回 OK 让 pipeline 继续，且不污染 ic（Keywords/Metadata 保持初始）。
func TestEnhancer_LLMFailure_ReturnsOKWithoutPolluting(t *testing.T) {
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return "", errors.New("openai 503")
	}}
	node := NewEnhancerNode(llm)
	ic := &IngestionContext{RawText: "原文"}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success, "LLM 失败时仍应返回 OK，让 pipeline 继续")
	require.True(t, res.ShouldContinue)
	assert.Nil(t, ic.Keywords, "失败时不应写入 Keywords")
	assert.Empty(t, ic.Metadata, "失败时不应写入 Metadata")
}

// TestEnhancer_JSONParseFailure_ReturnsOKWithoutPolluting 验证 LLM 返回非
// JSON（自然语言、空字符串等）时，节点优雅降级。
func TestEnhancer_JSONParseFailure_ReturnsOKWithoutPolluting(t *testing.T) {
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return "对不起我没读懂你的文档", nil
	}}
	node := NewEnhancerNode(llm)
	ic := &IngestionContext{RawText: "原文"}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success)
	assert.Nil(t, ic.Keywords)
	assert.Empty(t, ic.Metadata)
}

// TestEnhancer_MarkdownFenced_StripsAndParses 验证 LLM 把 JSON 包在 markdown
// ```json fence 里时，节点能正确剥离再解析（实际 LLM 经常这么干）。
func TestEnhancer_MarkdownFenced_StripsAndParses(t *testing.T) {
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return "```json\n{\"summary\":\"摘要\",\"keywords\":[\"a\"]}\n```", nil
	}}
	node := NewEnhancerNode(llm)
	ic := &IngestionContext{RawText: "原文"}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success)
	assert.Equal(t, []string{"a"}, ic.Keywords)
	assert.Equal(t, "摘要", ic.Metadata["doc_summary"])
}

// TestEnhancer_EmptyRawText_SkipsImmediately 验证 RawText 空时跳过 LLM 调用，
// 节省成本。
func TestEnhancer_EmptyRawText_SkipsImmediately(t *testing.T) {
	llmCalled := false
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		llmCalled = true
		return "", nil
	}}
	node := NewEnhancerNode(llm)
	ic := &IngestionContext{RawText: ""}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success)
	assert.False(t, llmCalled, "空 RawText 不应触发 LLM 调用")
}
