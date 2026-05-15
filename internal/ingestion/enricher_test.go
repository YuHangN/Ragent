package ingestion

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeChunks 构造 n 个简单 chunk，便于并发测试。
func makeChunks(n int) []VectorChunk {
	out := make([]VectorChunk, n)
	for i := 0; i < n; i++ {
		out[i] = VectorChunk{
			ChunkID:  "c" + string(rune('a'+i)),
			Index:    i,
			Content:  "chunk " + string(rune('a'+i)),
			Metadata: map[string]any{},
		}
	}
	return out
}

// TestEnricher_HappyPath_AllChunksEnriched 验证全部 chunk 成功时 EmbedText
// 拼接顺序正确（原文 → 摘要 → 问题），Metadata 字段也被填上。
func TestEnricher_HappyPath_AllChunksEnriched(t *testing.T) {
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return `{"summary":"摘要X","questions":["Q1","Q2"]}`, nil
	}}
	node := NewEnricherNode(llm, 2)
	ic := &IngestionContext{Chunks: makeChunks(3)}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success)
	require.True(t, res.ShouldContinue)
	for i, ch := range ic.Chunks {
		assert.Equal(t, "摘要X", ch.Metadata["summary"])
		assert.Equal(t, []string{"Q1", "Q2"}, ch.Metadata["questions"])
		// EmbedText 必须包含原文 + 摘要 + 问题（拼接顺序）
		assert.True(t, strings.HasPrefix(ch.EmbedText, ch.Content), "chunk %d: EmbedText 应以原文开头", i)
		assert.Contains(t, ch.EmbedText, "摘要X")
		assert.Contains(t, ch.EmbedText, "Q1；Q2")
	}
}

// TestEnricher_PartialFailure_OnlySkipsBadChunks 验证部分 chunk 的 LLM 失败时，
// 失败的 chunk 不写 EmbedText（→ Embedder 退回 Content），成功的正常增强，
// 且 pipeline 不被 Fail。
func TestEnricher_PartialFailure_OnlySkipsBadChunks(t *testing.T) {
	var idx atomic.Int32
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		// 第 2 次调用失败，其它成功
		if idx.Add(1) == 2 {
			return "", errors.New("rate limited")
		}
		return `{"summary":"S","questions":["Q"]}`, nil
	}}
	node := NewEnricherNode(llm, 1) // 并发=1 让调用顺序确定（chunk 0,1,2 依次）
	ic := &IngestionContext{Chunks: makeChunks(3)}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success, "部分 chunk 失败不应让节点 Fail")
	// chunk 0、2 成功（第 1、3 次调用），chunk 1 失败（第 2 次）
	assert.NotEmpty(t, ic.Chunks[0].EmbedText)
	assert.Empty(t, ic.Chunks[1].EmbedText, "失败的 chunk EmbedText 应为空")
	assert.NotEmpty(t, ic.Chunks[2].EmbedText)
	// 失败 chunk 的 Metadata 也应该没被写
	assert.Nil(t, ic.Chunks[1].Metadata["summary"])
}

// TestEnricher_AllFail_ReturnsOKWithZeroEnriched 验证全部失败时返回 OK
// （pipeline 继续），所有 chunk EmbedText 都为空。
func TestEnricher_AllFail_ReturnsOKWithZeroEnriched(t *testing.T) {
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return "", errors.New("openai 503")
	}}
	node := NewEnricherNode(llm, 4)
	ic := &IngestionContext{Chunks: makeChunks(3)}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success)
	require.True(t, res.ShouldContinue)
	for i, ch := range ic.Chunks {
		assert.Empty(t, ch.EmbedText, "chunk %d 全失败时 EmbedText 应为空", i)
	}
	assert.Contains(t, res.Message, "0/3")
}

// TestEnricher_EmptyChunks_SkipsImmediately 验证没有 chunk 时直接 OK 跳过，
// 不发起 LLM 调用。
func TestEnricher_EmptyChunks_SkipsImmediately(t *testing.T) {
	llmCalled := false
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		llmCalled = true
		return "", nil
	}}
	node := NewEnricherNode(llm, 4)
	ic := &IngestionContext{Chunks: nil}

	res := node.Execute(context.Background(), ic)

	require.True(t, res.Success)
	assert.False(t, llmCalled)
}

// TestEnricher_ConcurrencyLimit 验证并发上限生效——同时活跃的 goroutine 数
// 不超过 concurrency。用一个共享计数器观察峰值。
func TestEnricher_ConcurrencyLimit(t *testing.T) {
	const limit = 2
	var (
		active atomic.Int32
		peak   atomic.Int32
		mu     sync.Mutex
		hold   = make(chan struct{}) // 阻塞每次调用，逼出并发峰值
	)
	llm := &stubLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		now := active.Add(1)
		mu.Lock()
		if now > peak.Load() {
			peak.Store(now)
		}
		mu.Unlock()
		<-hold
		active.Add(-1)
		return `{"summary":"S","questions":["Q"]}`, nil
	}}
	node := NewEnricherNode(llm, limit)
	ic := &IngestionContext{Chunks: makeChunks(8)}

	done := make(chan NodeResult, 1)
	go func() { done <- node.Execute(context.Background(), ic) }()

	// 全部释放——errgroup.SetLimit 保证同时只有 limit 个 goroutine 在 onChat 里
	close(hold)
	<-done

	assert.LessOrEqual(t, int(peak.Load()), limit, "并发峰值不应超过 concurrency 上限")
}
