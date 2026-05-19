package intent

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/sync/errgroup"
)

type Resolver struct {
	classifier *Classifier
	// MaxIntents 是单个子问题最多保留的候选数。
	MaxIntents int
	// MinScore 是候选意图的最低分数门槛。
	MinScore float64
}

// NewResolver 创建意图解析器。
//
// maxIntents <= 0 时默认使用 3，表示每个子问题最多保留 3 个候选意图。
func NewResolver(classifier *Classifier, maxIntents int, minScore float64) *Resolver {
	if maxIntents <= 0 {
		maxIntents = 3
	}
	return &Resolver{classifier: classifier, MaxIntents: maxIntents, MinScore: minScore}
}

// Resolve 对每个子问题并行执行意图分类，返回结果按原顺序排列。
//
// 调用方负责保证 subQuestions 非空（通常用查询改写后的结果，至少含一个 fallback 文本）。
//
// 例子：subQuestions = ["介绍产品 A", "说明退款政策"]，会得到两个 SubQuestionIntent。
func (r *Resolver) Resolve(ctx context.Context, kbID int64, subQuestions []string) ([]SubQuestionIntent, error) {
	subs := subQuestions
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

// MergeGroup 把多子问题的候选列表合并为单个 Group。
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
func (r *Resolver) MergeGroup(subs []SubQuestionIntent) Group {
	bestByID := make(map[int64]Candidate)
	allSystemOnly := len(subs) > 0 // 没子问题不算 system_only

	for _, s := range subs {
		// 候选非空，且没有任何非 SYSTEM 候选，才算这个子问题是纯系统意图。
		thisSystemOnly := len(s.Candidates) > 0
		for _, c := range s.Candidates {
			if c.Kind != KindSystem {
				thisSystemOnly = false
				break
			}
		}
		if !thisSystemOnly {
			allSystemOnly = false
		}
		for _, c := range s.Candidates {
			if c.Kind == KindSystem {
				continue // SYSTEM 不进 KB/MCP 列表
			}
			if existing, ok := bestByID[c.NodeID]; !ok || c.Score > existing.Score {
				bestByID[c.NodeID] = c
			}
		}
	}

	var kb, mcp []Candidate
	for _, c := range bestByID {
		switch c.Kind {
		case KindKB:
			kb = append(kb, c)
		case KindMCP:
			mcp = append(mcp, c)
		}
	}
	sort.Slice(kb, func(i, j int) bool { return kb[i].Score > kb[j].Score })
	sort.Slice(mcp, func(i, j int) bool { return mcp[i].Score > mcp[j].Score })

	return Group{KbIntents: kb, McpIntents: mcp, AllSystemOnly: allSystemOnly}
}
