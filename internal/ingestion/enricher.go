// Package ingestion 提供文档摄入 pipeline。
//
// 本文件实现 EnricherNode——chunk 级 LLM 加工节点。它在 Chunker 之后、
// Embedder 之前执行，对每个 chunk 调一次 LLM 生成摘要与"能回答的问题"，
// 然后把这些信息拼进 chunk.EmbedText——这样 embedding 向量同时反映原文与
// 增强语义，而 chunk.Content 保持原文不变（业界 hypothetical-questions 模式）。
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// EnricherNode 对每个 chunk 做 LLM 加工。
//
// 并发由 concurrency 控制，避免一篇大文档瞬间打爆 LLM 配额（errgroup.SetLimit）。
// 失败策略：单个 chunk 的 LLM / JSON 失败只跳过该 chunk 的增强（EmbedText 留空 →
// Embedder 自动退回 Content），不影响其它 chunk，更不让 pipeline Fail。
type EnricherNode struct {
	llm         aiclient.LLMService
	concurrency int
}

// NewEnricherNode 构造 EnricherNode。concurrency <= 0 时默认 4。
func NewEnricherNode(llm aiclient.LLMService, concurrency int) *EnricherNode {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &EnricherNode{llm: llm, concurrency: concurrency}
}

// Name 返回节点名，用于 pipeline 日志。
func (n *EnricherNode) Name() string { return "enricher" }

// enricherOutput 是单 chunk LLM 返回 JSON 的解析目标。
type enricherOutput struct {
	Summary   string   `json:"summary"`
	Questions []string `json:"questions"`
}

// Execute 并发对所有 chunk 调 LLM，把生成的摘要 + 问题拼进 EmbedText。
//
// errgroup 内部 goroutine 永远返回 nil——失败只 log warn 然后跳过该 chunk，
// 不让 errgroup 把整组 cancel 掉。Wait 后通过扫描 EmbedText 非空数量得到
// 实际增强成功的 chunk 数，无需在并发循环里维护共享计数器（避免 race）。
func (n *EnricherNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if len(ic.Chunks) == 0 {
		return OK("enricher: 无 chunk，跳过")
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(n.concurrency)

	for i := range ic.Chunks {
		i := i
		g.Go(func() error {
			out, err := n.enrichOne(gctx, ic.Chunks[i].Content)
			if err != nil {
				zap.S().Warnf("enricher: chunk %d 增强失败: %v", i, err)
				return nil // 单 chunk 失败不取消其它 chunk
			}
			// 每个 goroutine 只写自己 index 的 chunk，不同内存位置写入安全
			if ic.Chunks[i].Metadata == nil {
				ic.Chunks[i].Metadata = make(map[string]any)
			}
			ic.Chunks[i].Metadata["summary"] = out.Summary
			ic.Chunks[i].Metadata["questions"] = out.Questions
			ic.Chunks[i].EmbedText = buildEmbedText(ic.Chunks[i].Content, out)
			return nil
		})
	}
	_ = g.Wait() // goroutine 内部已吞错，Wait 必定返回 nil

	enriched := 0
	for _, ch := range ic.Chunks {
		if ch.EmbedText != "" {
			enriched++
		}
	}
	return OK(fmt.Sprintf("enricher: 增强 %d/%d 个 chunk", enriched, len(ic.Chunks)))
}

// enrichOne 对单个 chunk 文本调一次 LLM。
func (n *EnricherNode) enrichOne(ctx context.Context, content string) (enricherOutput, error) {
	answer, err := n.llm.Chat(ctx, aiclient.ChatRequest{
		Messages: []aiclient.ChatMessage{
			aiclient.System(enricherSystemPrompt),
			aiclient.User(content),
		},
	})
	if err != nil {
		return enricherOutput{}, err
	}
	var out enricherOutput
	if err := json.Unmarshal([]byte(aiclient.StripMarkdownCodeFence(answer)), &out); err != nil {
		return enricherOutput{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return out, nil
}

// buildEmbedText 把原文 + 摘要 + 生成的问题拼成用于 embedding 的文本。
//
// 拼接顺序：原文在前（主体语义），增强信息在后（额外匹配入口）。这样 embedding
// 既保留原文的语义中心，又能匹配到与原文措辞不同的用户提问。
func buildEmbedText(content string, out enricherOutput) string {
	var sb strings.Builder
	sb.WriteString(content)
	if out.Summary != "" {
		sb.WriteString("\n摘要：")
		sb.WriteString(out.Summary)
	}
	if len(out.Questions) > 0 {
		sb.WriteString("\n相关问题：")
		sb.WriteString(strings.Join(out.Questions, "；"))
	}
	return sb.String()
}
