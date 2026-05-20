package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

const chunkRoutingPromptTemplate = `你是文档分类助手。给以下 N 个文档片段，每个判定它属于哪个意图节点（0.0-1.0 评分）。

意图节点列表：
{{nodes}}

文档片段列表（编号 1 到 N，每段独立判定）：
{{chunks}}

请返回 JSON 二维数组，第 i 项是第 i 个片段的评分列表，顺序严格匹配输入顺序。无相关候选时返回空数组 []。示例：
[
  [{"node_id": 1, "score": 0.9}],
  [{"node_id": 2, "score": 0.7}, {"node_id": 1, "score": 0.5}],
  []
]
只输出 JSON，不要任何其它文字。`

// ClassifyChunks 批量给文档片段打分。
//
// 与 Classify 区别：
//   - 输入是文档文本而非用户问题，用专用 prompt（chunkRoutingPromptTemplate）；
//   - 一次 LLM 调用处理 N 个片段，返回 [][]Candidate，第 i 项是第 i 个片段的候选；
//   - 同样按 minScore 过滤、按分数倒序取 topK；
//   - 不在 classifiable 列表中的 node_id 会被忽略。
//
// LLM 错误 / JSON 解析错误会直接返回 error，调用方（ChunkRouterNode）负责重试与回退。
func (c *Classifier) ClassifyChunks(ctx context.Context, kbID int64, contents []string, topK int, minScore float64) ([][]Candidate, error) {
	if len(contents) == 0 {
		return nil, nil
	}

	classifiable, err := c.intentRepo.FindClassifiableByKbID(kbID)
	if err != nil {
		return nil, fmt.Errorf("intent: load classifiable for kb %d: %w", kbID, err)
	}
	if len(classifiable) == 0 {
		// 没有可分类节点，每个 chunk 返回空候选——上游会全 fallback。
		return make([][]Candidate, len(contents)), nil
	}

	var nodeDescs []string
	for _, n := range classifiable {
		nodeDescs = append(nodeDescs,
			fmt.Sprintf("- ID=%d 类型=%s 名称=%s 描述=%s", n.ID, n.Kind, n.Name, n.Description))
	}

	var chunkLines []string
	for i, ct := range contents {
		// 单 chunk 文本可能很长；先做长度保护，避免 prompt 撑爆 LLM context。
		safe := ct
		if len([]rune(safe)) > 2000 {
			safe = string([]rune(safe)[:2000]) + "...(truncated)"
		}
		chunkLines = append(chunkLines, fmt.Sprintf("[%d] %s", i+1, safe))
	}

	prompt := chunkRoutingPromptTemplate
	prompt = strings.ReplaceAll(prompt, "{{nodes}}", strings.Join(nodeDescs, "\n"))
	prompt = strings.ReplaceAll(prompt, "{{chunks}}", strings.Join(chunkLines, "\n"))

	resp, err := c.llm.Chat(ctx, aiclient.ChatRequest{
		Messages: []aiclient.ChatMessage{aiclient.User(prompt)},
	})
	if err != nil {
		return nil, fmt.Errorf("intent: LLM classify chunks: %w", err)
	}

	cleaned := aiclient.StripMarkdownCodeFence(resp)

	var raw [][]struct {
		NodeID int64   `json:"node_id"`
		Score  float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("intent: parse chunk classify response: %w", err)
	}

	// LLM 返回长度可能不匹配输入。短了补空、长了截断，保持 caller 拿到的 slice 与 input 同长。
	if len(raw) < len(contents) {
		raw = append(raw, make([][]struct {
			NodeID int64   `json:"node_id"`
			Score  float64 `json:"score"`
		}, len(contents)-len(raw))...)
	}
	raw = raw[:len(contents)]

	nodeMap := make(map[int64]Node, len(classifiable))
	for _, n := range classifiable {
		nodeMap[n.ID] = n
	}

	results := make([][]Candidate, len(contents))
	for i, scores := range raw {
		var cands []Candidate
		for _, s := range scores {
			if s.Score < minScore {
				continue
			}
			n, ok := nodeMap[s.NodeID]
			if !ok {
				continue
			}
			cands = append(cands, Candidate{
				NodeID:        n.ID,
				NodeName:      n.Name,
				KbID:          n.KbID,
				Kind:          n.Kind,
				PartitionName: n.PartitionName,
				MCPToolID:     n.MCPToolID,
				Score:         s.Score,
			})
		}
		sort.Slice(cands, func(a, b int) bool { return cands[a].Score > cands[b].Score })
		if topK > 0 && len(cands) > topK {
			cands = cands[:topK]
		}
		results[i] = cands
	}

	return results, nil
}
