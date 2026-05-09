package rag

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/sync/errgroup"
)

type IntentResolver struct {
	classifier *IntentClassifier
	// MaxIntents 单子问题最多保留的候选数（对应 Java MAX_INTENT_COUNT）
	MaxIntents int
	// MinScore 候选最低分数门槛（对应 Java INTENT_MIN_SCORE）
	MinScore float64
}

func NewIntentResolver(classifier *IntentClassifier, maxIntents int, minScore float64) *IntentResolver {
	if maxIntents <= 0 {
		maxIntents = 3
	}
	return &IntentResolver{classifier: classifier, MaxIntents: maxIntents, MinScore: minScore}
}

// Resolve 对 RewriteResult 中的所有子问题并行执行意图分类，
func (r *IntentResolver) Resolve(ctx context.Context, kbID int64, rewrite RewriteResult) ([]SubQuestionIntent, error) {
	subs := rewrite.SubQuestions
	if len(subs) == 0 {
		subs = []string{rewrite.RewrittenQuery}
	}

	results := make([]SubQuestionIntent, len(subs))
	g, gctx := errgroup.WithContext(ctx)

	for i, sub := range subs {
		i, sub := i, sub
		g.Go(func() error {
			candidates, err := r.classifier.Classify(gctx, kbID, sub, r.MaxIntents, r.MinScore)
			if err != nil {
				return fmt.Errorf("resolve sub %q: %w", sub, err)
			}
			results[i] = SubQuestionIntent{SubQuestion: sub, Candidates: candidates}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// MergeGroup 把多子问题的候选列表合并为单个 IntentGroup。
//
// 关键规则（语义对齐 Java IntentResolver.isSystemOnly + RAGChatServiceImpl，
// 但修正 Java `size == 1` 的保守 bug：多个 SYSTEM 候选并存仍算 system_only）：
//   - 同一 NodeID 在多个子问题里出现时取最高分
//   - SYSTEM 候选不进 KbIntents / McpIntents
//   - AllSystemOnly = 所有子问题都满足"候选非空 且 全部候选 Kind==SYSTEM"
//     混合 SYSTEM+KB/MCP 不短路；纯 SYSTEM（含多候选）才短路。
func (r *IntentResolver) MergeGroup(subs []SubQuestionIntent) IntentGroup {
	bestByID := make(map[int64]IntentCandidate)
	allSystemOnly := len(subs) > 0 // 没子问题不算 system_only

	for _, s := range subs {
		// 当前子问题是否"全部候选都是 SYSTEM"（候选非空 + 没有非-SYSTEM 类型）
		thisSystemOnly := len(s.Candidates) > 0
		for _, c := range s.Candidates {
			if c.Kind != IntentKindSystem {
				thisSystemOnly = false
				break
			}
		}
		if !thisSystemOnly {
			allSystemOnly = false
		}
		for _, c := range s.Candidates {
			if c.Kind == IntentKindSystem {
				continue // SYSTEM 不进 KB/MCP 列表
			}
			if existing, ok := bestByID[c.NodeID]; !ok || c.Score > existing.Score {
				bestByID[c.NodeID] = c
			}
		}
	}

	var kb, mcp []IntentCandidate
	for _, c := range bestByID {
		switch c.Kind {
		case IntentKindKB:
			kb = append(kb, c)
		case IntentKindMCP:
			mcp = append(mcp, c)
		}
	}
	sort.Slice(kb, func(i, j int) bool { return kb[i].Score > kb[j].Score })
	sort.Slice(mcp, func(i, j int) bool { return mcp[i].Score > mcp[j].Score })

	return IntentGroup{KbIntents: kb, McpIntents: mcp, AllSystemOnly: allSystemOnly}
}
