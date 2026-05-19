package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeGroup_DedupKeepHighestScore(t *testing.T) {
	r := &IntentResolver{}
	subs := []SubQuestionIntent{
		{SubQuestion: "Q1", Candidates: []IntentCandidate{
			{NodeID: 1, Kind: IntentKindKB, PartitionName: "p1", Score: 0.7},
		}},
		{SubQuestion: "Q2", Candidates: []IntentCandidate{
			{NodeID: 1, Kind: IntentKindKB, PartitionName: "p1", Score: 0.9}, // 同 NodeID 高分
			{NodeID: 2, Kind: IntentKindMCP, MCPToolID: "tool_x", Score: 0.6},
		}},
	}

	g := r.MergeGroup(subs)
	assert.Len(t, g.KbIntents, 1)
	assert.InDelta(t, 0.9, g.KbIntents[0].Score, 0.001)
	assert.Len(t, g.McpIntents, 1)
	assert.False(t, g.AllSystemOnly)
}

func TestMergeGroup_AllSystemOnly_PureSystem(t *testing.T) {
	// 所有子问题都仅命中单个 SYSTEM 候选 → 短路
	r := &IntentResolver{}
	subs := []SubQuestionIntent{
		{SubQuestion: "你好", Candidates: []IntentCandidate{
			{NodeID: 99, Kind: IntentKindSystem, Score: 0.95},
		}},
	}
	g := r.MergeGroup(subs)
	assert.True(t, g.AllSystemOnly)
	assert.Empty(t, g.KbIntents)
	assert.Empty(t, g.McpIntents)
}

func TestMergeGroup_AllSystemOnly_MixedKbAndSystem_NotShortCircuit(t *testing.T) {
	// 关键场景：用户问 "你好，介绍一下产品"
	//   子问题 1 "你好" 命中 SYSTEM
	//   子问题 2 "介绍产品" 命中 KB
	// 期望：AllSystemOnly=false，仍走 KB 检索。
	r := &IntentResolver{}
	subs := []SubQuestionIntent{
		{SubQuestion: "你好", Candidates: []IntentCandidate{
			{NodeID: 99, Kind: IntentKindSystem, Score: 0.9},
		}},
		{SubQuestion: "介绍产品", Candidates: []IntentCandidate{
			{NodeID: 5, Kind: IntentKindKB, PartitionName: "p5", Score: 0.85},
		}},
	}
	g := r.MergeGroup(subs)
	assert.False(t, g.AllSystemOnly, "混合 SYSTEM+KB 时不应短路")
	assert.Len(t, g.KbIntents, 1)
	assert.Equal(t, int64(5), g.KbIntents[0].NodeID)
}

func TestMergeGroup_AllSystemOnly_MultiSystemCandidates_StillShortCircuits(t *testing.T) {
	// 单子问题内命中多个 SYSTEM 候选（如同时命中"问候"和"自我介绍"）
	// → 仍然算 system_only。修正了 Java size==1 的保守 bug。
	r := &IntentResolver{}
	subs := []SubQuestionIntent{
		{SubQuestion: "你好，介绍一下你自己", Candidates: []IntentCandidate{
			{NodeID: 99, Kind: IntentKindSystem, Score: 0.85}, // 问候
			{NodeID: 98, Kind: IntentKindSystem, Score: 0.80}, // 自我介绍
		}},
	}
	g := r.MergeGroup(subs)
	assert.True(t, g.AllSystemOnly, "纯系统问题命中多个 SYSTEM 仍应短路")
	assert.Empty(t, g.KbIntents)
	assert.Empty(t, g.McpIntents)
}

func TestMergeGroup_AllSystemOnly_SystemPlusKbInOneSubQuestion_NotShortCircuit(t *testing.T) {
	// 单子问题内 SYSTEM 与 KB 并存 → 不短路（用户隐含了知识库需求）
	r := &IntentResolver{}
	subs := []SubQuestionIntent{
		{SubQuestion: "Q", Candidates: []IntentCandidate{
			{NodeID: 99, Kind: IntentKindSystem, Score: 0.7},
			{NodeID: 5, Kind: IntentKindKB, PartitionName: "p5", Score: 0.6},
		}},
	}
	g := r.MergeGroup(subs)
	assert.False(t, g.AllSystemOnly)
	assert.Len(t, g.KbIntents, 1)
}

func TestMergeGroup_EmptySubs(t *testing.T) {
	r := &IntentResolver{}
	g := r.MergeGroup(nil)
	assert.False(t, g.AllSystemOnly, "无子问题不应触发系统短路")
	assert.Empty(t, g.KbIntents)
}
