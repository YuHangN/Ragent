// Package ingestion 提供文档摄入 pipeline。
//
// 本文件实现 EnhancerNode——文档级 LLM 加工节点。它在 Parser 之后、Chunker
// 之前执行，对整篇文档调一次 LLM，抽取摘要与关键词写入 IngestionContext，
// 供下游 EnricherNode / 检索阶段参考。
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"go.uber.org/zap"
)

// EnhancerNode 对整篇文档做一次 LLM 加工。
//
// 失败策略：LLM 调用或 JSON 解析出错时**不让 pipeline 失败**——enrichment 是
// 增强而不是硬需求，log warn 后带原始数据继续。Chunker 不依赖 Enhancer 的输出
// （只读 RawText / EnhancedText），所以跳过增强不会破坏下游。
type EnhancerNode struct {
	llm aiclient.LLMService
}

// NewEnhancerNode 构造 EnhancerNode。
func NewEnhancerNode(llm aiclient.LLMService) *EnhancerNode {
	return &EnhancerNode{llm: llm}
}

// Name 返回节点名，用于 pipeline 日志。
func (n *EnhancerNode) Name() string { return "enhancer" }

// enhancerOutput 是 LLM 返回 JSON 的解析目标。
type enhancerOutput struct {
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
}

// Execute 调 LLM 抽取文档级摘要 + 关键词。
//
// 任意失败都返回 OK——pipeline 继续，下游用 RawText 跑就行。成功时把结果写到
// ic.Keywords 和 ic.Metadata["doc_summary"]。
func (n *EnhancerNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if ic.RawText == "" {
		return OK("enhancer: 无文本，跳过")
	}

	answer, err := n.llm.Chat(ctx, aiclient.ChatRequest{
		Messages: []aiclient.ChatMessage{
			aiclient.System(enhancerSystemPrompt),
			aiclient.User(ic.RawText),
		},
	})
	if err != nil {
		zap.S().Warnf("enhancer: LLM 调用失败，跳过文档增强: %v", err)
		return OK("enhancer: LLM 失败，跳过")
	}

	var out enhancerOutput
	if err := json.Unmarshal([]byte(aiclient.StripMarkdownCodeFence(answer)), &out); err != nil {
		zap.S().Warnf("enhancer: JSON 解析失败，跳过文档增强: %v", err)
		return OK("enhancer: 解析失败，跳过")
	}

	if ic.Metadata == nil {
		ic.Metadata = make(map[string]any)
	}
	ic.Metadata["doc_summary"] = out.Summary
	ic.Keywords = out.Keywords

	return OK(fmt.Sprintf("enhancer: 抽取 %d 个关键词", len(out.Keywords)))
}
