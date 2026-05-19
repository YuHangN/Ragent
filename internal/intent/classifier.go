package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

const intentClassifyPromptTemplate = `你是一个意图分类助手。请对用户问题与以下每个意图节点的相关性进行评分（0.0-1.0）。

用户问题：{{question}}

意图节点列表：
{{nodes}}

请以 JSON 数组格式回复，不要输出任何其他内容：
[{"node_id": 1, "score": 0.85}, {"node_id": 2, "score": 0.3}]`

// Classifier 使用 LLM 给意图节点打分。
//
// 它不负责拆分问题，也不负责决定后续走哪个检索通道。它只做一件事：
// 给某个 KB 下“可分类的意图节点”计算相关性分数，并转成 Candidate。
//
// 例子：用户问题是“产品 A 怎么安装？”，KB 下有三个叶子意图：
//   - 产品安装
//   - 售后退款
//   - 闲聊问候
//
// LLM 可能返回安装 0.92、退款 0.20、问候 0.05。Classify 会按 minScore 和 topK
// 过滤后，只返回分数足够高的候选。
type Classifier struct {
	llm        aiclient.LLMService
	intentRepo Repo
}

// NewClassifier 创建意图分类器。
//
// llm 用来判断“问题和意图节点有多相关”；repo 用来加载指定 KB 下可分类的意图节点。
func NewClassifier(llm aiclient.LLMService, repo Repo) *Classifier {
	return &Classifier{llm: llm, intentRepo: repo}
}

// Classify 对指定 KB 下的可分类意图节点打分。
//
// 执行流程：
//   - 从 intentRepo 读取该 KB 下可分类的节点。当前仓库实现只返回启用的叶子节点。
//   - 把问题和节点列表拼进 prompt，让 LLM 返回 [{"node_id": ..., "score": ...}]。
//   - 清理 LLM 可能包上的 markdown 代码块，再解析 JSON。
//   - 丢弃低于 minScore 的结果，也丢弃仓库中不存在的 node_id。
//   - 把分数合格的 Node 转成 Candidate，并按分数降序返回前 topK 个。
//
// 例子：kbID=100、question="产品 A 怎么安装？"、topK=2、minScore=0.5。
// 仓库中有三个节点：
//   - ID=1，Kind=KB，Name="产品安装"，PartitionName="install"
//   - ID=2，Kind=KB，Name="退款政策"，PartitionName="refund"
//   - ID=3，Kind=SYSTEM，Name="闲聊问候"
//
// 如果 LLM 返回：
//   - [{"node_id":1,"score":0.9},{"node_id":2,"score":0.4},{"node_id":999,"score":0.8}]
//
// 最终只返回 ID=1。ID=2 分数低于 minScore，ID=999 不在仓库节点列表中。
//
// 返回的候选会带上 Kind / PartitionName / MCPToolID。调用方可以根据 Kind
// 决定后续是走 KB 检索（用 PartitionName 缩范围）、系统回复，还是外部工具。
func (c *Classifier) Classify(ctx context.Context, kbID int64, question string, topK int, minScore float64) ([]Candidate, error) {
	classifiable, err := c.intentRepo.FindClassifiableByKbID(kbID)
	if err != nil {
		return nil, fmt.Errorf("intent: load classifiable for kb %d: %w", kbID, err)
	}
	if len(classifiable) == 0 {
		return nil, nil
	}

	// 构建节点列表描述，供 LLM 判断每个节点和问题的相关性。
	// Kind 一起传给 LLM，避免把 SYSTEM / MCP / KB 节点混成同一种语义。
	var nodeDescs []string
	for _, n := range classifiable {
		nodeDescs = append(nodeDescs,
			fmt.Sprintf("- ID=%d 类型=%s 名称=%s 描述=%s", n.ID, n.Kind, n.Name, n.Description))
	}

	prompt := intentClassifyPromptTemplate
	prompt = strings.ReplaceAll(prompt, "{{question}}", question)
	prompt = strings.ReplaceAll(prompt, "{{nodes}}", strings.Join(nodeDescs, "\n"))

	resp, err := c.llm.Chat(ctx, aiclient.ChatRequest{
		Messages: []aiclient.ChatMessage{aiclient.User(prompt)},
	})
	if err != nil {
		return nil, fmt.Errorf("intent: LLM classify: %w", err)
	}

	cleaned := aiclient.StripMarkdownCodeFence(resp)

	// LLM 应该只返回 JSON 数组；如果外层包了 ```json，前面已经清理掉。
	var scores []struct {
		NodeID int64   `json:"node_id"`
		Score  float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(cleaned), &scores); err != nil {
		return nil, fmt.Errorf("intent: parse LLM scores: %w", err)
	}

	nodeMap := make(map[int64]Node, len(classifiable))
	for _, n := range classifiable {
		nodeMap[n.ID] = n
	}

	// 只信任当前 KB 中真实存在的节点。LLM 返回的未知 node_id 会被忽略。
	var candidates []Candidate
	for _, s := range scores {
		if s.Score < minScore {
			continue
		}
		n, ok := nodeMap[s.NodeID]
		if !ok {
			continue
		}
		candidates = append(candidates, Candidate{
			NodeID:         n.ID,
			NodeName:       n.Name,
			KbID:           n.KbID,
			Kind:           n.Kind,
			PartitionName: n.PartitionName, // 节点自带，无需回溯
			MCPToolID:      n.MCPToolID,
			Score:          s.Score,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if topK > 0 && len(candidates) > topK {
		candidates = candidates[:topK]
	}

	return candidates, nil
}
