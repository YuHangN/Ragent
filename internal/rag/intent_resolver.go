package rag

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/sync/errgroup"
)

type IntentResolver struct {
	classifier *IntentClassifier
	// MaxIntents 是单个子问题最多保留的候选数。
	MaxIntents int
	// MinScore 是候选意图的最低分数门槛。
	MinScore float64
}

// NewIntentResolver 创建意图解析器。
//
// maxIntents <= 0 时默认使用 3，表示每个子问题最多保留 3 个候选意图。
func NewIntentResolver(classifier *IntentClassifier, maxIntents int, minScore float64) *IntentResolver {
	if maxIntents <= 0 {
		maxIntents = 3
	}
	return &IntentResolver{classifier: classifier, MaxIntents: maxIntents, MinScore: minScore}
}

// Resolve 对改写结果中的每个子问题执行意图分类。
//
// 如果 rewrite.SubQuestions 为空，会退回使用 rewrite.RewrittenQuery，保证至少有一个
// 查询文本参与分类。多个子问题会并行分类，返回结果仍按原子问题顺序排列。
//
// 例子：
//   - RewrittenQuery="介绍产品 A，并说明退款政策"
//   - SubQuestions=["介绍产品 A", "说明退款政策"]
//
// Resolve 会分别调用 classifier.Classify，并返回两个 SubQuestionIntent。
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
// 合并规则：
//   - 同一个 NodeID 在多个子问题中出现时，只保留最高分。
//   - SYSTEM 候选不放进 KbIntents 或 McpIntents。
//   - 只有所有子问题都“有候选，并且候选全是 SYSTEM”时，AllSystemOnly 才为 true。
//
// 例子 1：两个子问题都命中同一个 KB 节点，分数分别是 0.7 和 0.9，
// 合并后只保留 0.9 的那个候选。
//
// 例子 2：“你好，介绍一下产品”被拆成“你好”和“介绍产品”：
//   - “你好”命中 SYSTEM。
//   - “介绍产品”命中 KB。
//
// 这不是纯系统问题，AllSystemOnly=false，后续仍会走 KB 检索。
func (r *IntentResolver) MergeGroup(subs []SubQuestionIntent) IntentGroup {
	bestByID := make(map[int64]IntentCandidate)
	allSystemOnly := len(subs) > 0 // 没子问题不算 system_only

	for _, s := range subs {
		// 候选非空，且没有任何非 SYSTEM 候选，才算这个子问题是纯系统意图。
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
