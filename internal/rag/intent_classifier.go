package rag

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

// IntentClassifier 使用 LLM 对意图节点列表打分，返回按分数降序排列的候选列表。
type IntentClassifier struct {
	llm        aiclient.LLMService
	intentRepo IntentRepo
}

func NewIntentClassifier(llm aiclient.LLMService, repo IntentRepo) *IntentClassifier {
	return &IntentClassifier{llm: llm, intentRepo: repo}
}

// Classify 对指定知识库下所有可分类的意图节点打分，
// 返回分数超过 minScore 的 Top-K 候选（按分数降序）。
// 候选已带上节点的 Kind / CollectionName / MCPToolID，调用方按 Kind 分流。
func (c *IntentClassifier) Classify(ctx context.Context, kbID int64, question string, topK int, minScore float64) ([]IntentCandidate, error) {
	classifiable, err := c.intentRepo.FindClassifiableByKbID(kbID)
	if err != nil {
		return nil, fmt.Errorf("intent: load classifiable for kb %d: %w", kbID, err)
	}
	if len(classifiable) == 0 {
		return nil, nil
	}

	// 构建节点列表描述，供 LLM 参考。带上 Kind 让 LLM 理解节点性质。
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

	var scores []struct {
		NodeID int64   `json:"node_id"`
		Score  float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(cleaned), &scores); err != nil {
		return nil, fmt.Errorf("intent: parse LLM scores: %w", err)
	}

	nodeMap := make(map[int64]IntentNode, len(classifiable))
	for _, n := range classifiable {
		nodeMap[n.ID] = n
	}

	var candidates []IntentCandidate
	for _, s := range scores {
		if s.Score < minScore {
			continue
		}
		n, ok := nodeMap[s.NodeID]
		if !ok {
			continue
		}
		candidates = append(candidates, IntentCandidate{
			NodeID:         n.ID,
			NodeName:       n.Name,
			KbID:           n.KbID,
			Kind:           n.Kind,
			CollectionName: n.CollectionName, // 节点自带，无需回溯
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
