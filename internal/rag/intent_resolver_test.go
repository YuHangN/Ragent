package rag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeGroup_DedupKeepHighestScore(t *testing.T) {
	r := &IntentResolver{}
	subs := []SubQuestionIntent{
		{SubQuestion: "Q1", Candidates: []IntentCandidate{
			{NodeID: 1, Kind: IntentKindKB, CollectionName: "kb_1", Score: 0.7},
		}},
		{SubQuestion: "Q2", Candidates: []IntentCandidate{
			{NodeID: 1, Kind: IntentKindKB, CollectionName: "kb_1", Score: 0.9}, // 同 NodeID 高分
			{NodeID: 2, Kind: IntentKindMCP, MCPToolID: "tool_x", Score: 0.6},
		}},
	}

	g := r.MergeGroup(subs)
	assert.Len(t, g.KbIntents, 1)
	assert.InDelta(t, 0.9, g.KbIntents[0].Score, 0.001)
	assert.Len(t, g.McpIntents, 1)
	assert.False(t, g.HasSystem)
}

func TestMergeGroup_SystemFlag(t *testing.T) {
	r := &IntentResolver{}
	subs := []SubQuestionIntent{
		{SubQuestion: "你好", Candidates: []IntentCandidate{
			{NodeID: 99, Kind: IntentKindSystem, Score: 0.95},
		}},
	}
	g := r.MergeGroup(subs)
	assert.True(t, g.HasSystem)
	assert.Empty(t, g.KbIntents)
	assert.Empty(t, g.McpIntents)
}
