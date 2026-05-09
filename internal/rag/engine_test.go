package rag

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
