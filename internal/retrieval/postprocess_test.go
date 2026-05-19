package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeduplication_RemovesDuplicates(t *testing.T) {
	proc := &DeduplicationProcessor{}

	results := []SearchChannelResult{
		{ChannelName: "intent_directed", Priority: 1, Chunks: []RetrievedChunk{
			{ID: "a", Content: "内容A", Score: 0.9},
			{ID: "b", Content: "内容B", Score: 0.8},
		}},
		{ChannelName: "vector_global", Priority: 10, Chunks: []RetrievedChunk{
			{ID: "a", Content: "内容A", Score: 0.7}, // 与高优先级通道重复
			{ID: "c", Content: "内容C", Score: 0.6},
		}},
	}

	deduped := proc.Process(nil, results, SearchContext{})
	require.Len(t, deduped, 3)
	// a 应保留最高分 0.9（来自高优先级通道）
	for _, c := range deduped {
		if c.ID == "a" {
			assert.InDelta(t, 0.9, c.Score, 0.001)
		}
	}
}

func TestDeduplication_Empty(t *testing.T) {
	proc := &DeduplicationProcessor{}
	result := proc.Process(nil, nil, SearchContext{})
	assert.Empty(t, result)
}
