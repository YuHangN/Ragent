package retrieval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChannel struct {
	name     string
	priority int
	enabled  bool
	chunks   []RetrievedChunk
}

func (m *mockChannel) Name() string                   { return m.name }
func (m *mockChannel) Priority() int                  { return m.priority }
func (m *mockChannel) IsEnabled(_ SearchContext) bool { return m.enabled }
func (m *mockChannel) Search(_ context.Context, _ SearchContext) (SearchChannelResult, error) {
	return SearchChannelResult{
		ChannelName: m.name,
		Priority:    m.priority,
		Chunks:      m.chunks,
		Confidence:  0.8,
	}, nil
}

func TestMultiChannelEngine_BasicRetrieve(t *testing.T) {
	ch1 := &mockChannel{name: "ch1", priority: 1, enabled: true, chunks: []RetrievedChunk{
		{ID: "a", Content: "内容A", Score: 0.9},
	}}
	ch2 := &mockChannel{name: "ch2", priority: 10, enabled: true, chunks: []RetrievedChunk{
		{ID: "b", Content: "内容B", Score: 0.7},
		{ID: "a", Content: "内容A", Score: 0.6}, // 重复，应被去重
	}}

	engine := NewMultiChannelEngine(
		[]SearchChannel{ch1, ch2},
		[]PostProcessor{&DeduplicationProcessor{}},
	)

	chunks, err := engine.Retrieve(context.Background(), SearchContext{Question: "测试", TopK: 5})
	require.NoError(t, err)
	assert.Len(t, chunks, 2) // 去重后只剩 a 和 b
}

func TestMultiChannelEngine_DisabledChannel(t *testing.T) {
	ch := &mockChannel{name: "ch", priority: 1, enabled: false, chunks: []RetrievedChunk{
		{ID: "a", Content: "内容A"},
	}}
	engine := NewMultiChannelEngine([]SearchChannel{ch}, nil)
	chunks, err := engine.Retrieve(context.Background(), SearchContext{Question: "测试", TopK: 5})
	require.NoError(t, err)
	assert.Empty(t, chunks)
}

// tierAwareChannel 是 Phase 6.7.1 测试用的 mock，能感知 sc.PriorTierEmptySubQuestions
// 并报告 PerSubQuestionHits，模拟 IntentDirected / VectorGlobal 真实交互。
type tierAwareChannel struct {
	name           string
	priority       int
	isEnabledFn    func(sc SearchContext) bool
	searchFn       func(sc SearchContext) SearchChannelResult
	gotSC          SearchContext // 记录 Search 时收到的 sc，供断言用
	searchCalled   bool
}

func (m *tierAwareChannel) Name() string  { return m.name }
func (m *tierAwareChannel) Priority() int { return m.priority }
func (m *tierAwareChannel) IsEnabled(sc SearchContext) bool {
	return m.isEnabledFn(sc)
}
func (m *tierAwareChannel) Search(_ context.Context, sc SearchContext) (SearchChannelResult, error) {
	m.searchCalled = true
	m.gotSC = sc
	return m.searchFn(sc), nil
}

// TestEngine_HighScoreIntentWithHits_NoFallthrough 验证 happy path：
// IntentDirected 高分意图查到东西 → VectorGlobal 不该被触发。
func TestEngine_HighScoreIntentWithHits_NoFallthrough(t *testing.T) {
	direct := &tierAwareChannel{
		name: "direct", priority: 1,
		isEnabledFn: func(_ SearchContext) bool { return true },
		searchFn: func(_ SearchContext) SearchChannelResult {
			return SearchChannelResult{
				ChannelName:        "direct",
				Priority:           1,
				Chunks:             []RetrievedChunk{{ID: "x", Score: 0.9}},
				PerSubQuestionHits: map[string]int{"Q1": 1}, // 有命中
			}
		},
	}
	global := &tierAwareChannel{
		name: "global", priority: 10,
		// 模拟真实 VectorGlobal：所有子问题都被高分意图覆盖 + 前一 tier 没查空 → 关掉
		isEnabledFn: func(sc SearchContext) bool {
			for _, sq := range sc.SubIntents {
				if sc.PriorTierEmptySubQuestions[sq.SubQuestion] {
					return true
				}
			}
			return false
		},
		searchFn: func(_ SearchContext) SearchChannelResult {
			return SearchChannelResult{ChannelName: "global", Priority: 10}
		},
	}
	engine := NewMultiChannelEngine([]SearchChannel{direct, global}, nil)

	sc := SearchContext{
		Question: "Q1",
		SubIntents: []SubQuestionIntent{
			{SubQuestion: "Q1", Candidates: []IntentCandidate{
				{Kind: IntentKindKB, Score: 0.95, PartitionName: "refund"},
			}},
		},
		TopK: 5,
	}
	chunks, err := engine.Retrieve(context.Background(), sc)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.False(t, global.searchCalled, "happy path 下 VectorGlobal 不应被调用")
}

// TestEngine_HighScoreIntentEmptyPartition_FallthroughToGlobal 验证 Phase 6.7.1 修的核心场景：
// IntentDirected 高分意图查空（典型：文档没填 targetPartition）→ engine 把子问题填到
// PriorTierEmptySubQuestions → VectorGlobal IsEnabled 返回 true → 兜底跑起来。
func TestEngine_HighScoreIntentEmptyPartition_FallthroughToGlobal(t *testing.T) {
	direct := &tierAwareChannel{
		name: "direct", priority: 1,
		isEnabledFn: func(_ SearchContext) bool { return true },
		searchFn: func(_ SearchContext) SearchChannelResult {
			return SearchChannelResult{
				ChannelName:        "direct",
				Priority:           1,
				Chunks:             nil,
				PerSubQuestionHits: map[string]int{"Q1": 0}, // 关键：跑过但 0 命中
			}
		},
	}
	global := &tierAwareChannel{
		name: "global", priority: 10,
		isEnabledFn: func(sc SearchContext) bool {
			for _, sq := range sc.SubIntents {
				if sc.PriorTierEmptySubQuestions[sq.SubQuestion] {
					return true
				}
			}
			return false
		},
		searchFn: func(_ SearchContext) SearchChannelResult {
			return SearchChannelResult{
				ChannelName: "global",
				Priority:    10,
				Chunks:      []RetrievedChunk{{ID: "y", Score: 0.7}}, // 兜底命中
			}
		},
	}
	engine := NewMultiChannelEngine([]SearchChannel{direct, global}, nil)

	sc := SearchContext{
		Question: "Q1",
		SubIntents: []SubQuestionIntent{
			{SubQuestion: "Q1", Candidates: []IntentCandidate{
				{Kind: IntentKindKB, Score: 0.95, PartitionName: "refund"},
			}},
		},
		TopK: 5,
	}
	chunks, err := engine.Retrieve(context.Background(), sc)
	require.NoError(t, err)
	assert.True(t, global.searchCalled, "IntentDirected 查空时 VectorGlobal 必须兜底")
	assert.True(t, global.gotSC.PriorTierEmptySubQuestions["Q1"], "engine 应把空子问题注入下一 tier")
	require.Len(t, chunks, 1)
	assert.Equal(t, "y", chunks[0].ID)
}

// TestEngine_AllSystemOnly_NoFallthroughEvenIfEmpty 验证：AllSystemOnly 场景仍然按原
// 设计完全短路（不查 KB），即使前一 tier 没命中也不应触发兜底。
func TestEngine_AllSystemOnly_NoFallthroughEvenIfEmpty(t *testing.T) {
	direct := &tierAwareChannel{
		name: "direct", priority: 1,
		isEnabledFn: func(_ SearchContext) bool { return false }, // SYSTEM 下两者都关
		searchFn: func(_ SearchContext) SearchChannelResult {
			return SearchChannelResult{}
		},
	}
	global := &tierAwareChannel{
		name: "global", priority: 10,
		isEnabledFn: func(sc SearchContext) bool {
			if sc.IntentGroup.AllSystemOnly {
				return false
			}
			for _, sq := range sc.SubIntents {
				if sc.PriorTierEmptySubQuestions[sq.SubQuestion] {
					return true
				}
			}
			return false
		},
		searchFn: func(_ SearchContext) SearchChannelResult {
			return SearchChannelResult{}
		},
	}
	engine := NewMultiChannelEngine([]SearchChannel{direct, global}, nil)

	sc := SearchContext{
		Question:    "你好",
		SubIntents:  []SubQuestionIntent{{SubQuestion: "你好"}},
		IntentGroup: IntentGroup{AllSystemOnly: true},
		TopK:        5,
	}
	chunks, err := engine.Retrieve(context.Background(), sc)
	require.NoError(t, err)
	assert.Empty(t, chunks)
	assert.False(t, direct.searchCalled, "AllSystemOnly 下 IntentDirected 不该跑")
	assert.False(t, global.searchCalled, "AllSystemOnly 下 VectorGlobal 不该跑")
}
