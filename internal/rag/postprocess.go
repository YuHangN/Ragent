// Package rag 包含检索增强生成（RAG）相关的核心业务逻辑。
//
// 本文件定义检索结果的后处理流程。前面的检索通道可能会从多个知识库、多个 collection
// 返回候选片段，这些片段可能重复、顺序也未必最适合直接交给大模型。因此这里会在答案
// 生成前，对候选片段做去重、分数合并、重排序和 TopK 截断。
//
// 通俗来说：这个文件负责把“多个检索通道捞回来的原始资料”，整理成“一份更干净、更相关、
// 数量合适的上下文材料”，再交给后续 RAG 流程使用。
package rag

import (
	"context"
	"sort"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// DeduplicationProcessor 是检索结果去重处理器。
//
// 多个检索通道可能会命中同一个文档片段。该处理器会按 chunk.ID 去重，并保留同一片段
// 在不同通道中的最高分数，避免重复内容挤占后续 RAG 上下文窗口。
type DeduplicationProcessor struct{}

// Order 返回处理器执行顺序。
//
// 去重应该尽早执行，先减少候选片段数量，再交给 rerank 等更昂贵的后处理步骤。
func (p *DeduplicationProcessor) Order() int { return 1 }

// Process 按通道优先级合并并去重检索结果。
//
// 参数 chunks 是上一阶段传入的候选片段；results 是各检索通道的原始输出。这里以
// results 为准重新合并，因为通道结果包含 Priority，可保证高优先级通道先进入结果集。
// 如果同一个 chunk.ID 重复出现，只更新为更高的分数，不重复追加内容。
func (p *DeduplicationProcessor) Process(chunks []RetrievedChunk, results []SearchChannelResult, _ SearchContext) []RetrievedChunk {
	// 按通道优先级排序（Priority 越小越先处理，高优先级结果不被低优先级覆盖）
	sorted := make([]SearchChannelResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	seen := make(map[string]int) // chunk.ID → index in deduped
	var deduped []RetrievedChunk

	for _, result := range sorted {
		for _, chunk := range result.Chunks {
			if idx, exists := seen[chunk.ID]; exists {
				// 已存在：保留最高分
				if chunk.Score > deduped[idx].Score {
					deduped[idx].Score = chunk.Score
				}
			} else {
				seen[chunk.ID] = len(deduped)
				deduped = append(deduped, chunk)
			}
		}
	}

	return deduped
}

// RerankProcessor 使用 Rerank 模型对去重后的候选片段重新排序。
//
// 向量检索分数只能表示粗略语义相似度，rerank 会结合原始问题对候选片段做更精细的相关性
// 判断，并截取最终 TopK，减少无关片段进入大模型上下文。
type RerankProcessor struct {
	rerank aiclient.RerankService
}

// NewRerankProcessor 创建 Rerank 后处理器。
//
// rerank 是外部 AI rerank 服务，用于根据用户问题重新计算候选片段相关性。
func NewRerankProcessor(rerank aiclient.RerankService) *RerankProcessor {
	return &RerankProcessor{rerank: rerank}
}

// Order 返回处理器执行顺序。
//
// rerank 依赖去重后的候选片段，因此顺序应晚于 DeduplicationProcessor。
func (p *RerankProcessor) Order() int { return 10 }

// Process 调用 rerank 服务重排候选片段，并返回最终排序结果。
//
// 如果 rerank 服务失败，流程会自动降级为按原始检索分数倒序排序并截取 TopK，保证检索
// 链路可用性，不会因为重排模型异常而导致整个 RAG 请求失败。
func (p *RerankProcessor) Process(chunks []RetrievedChunk, _ []SearchChannelResult, sc SearchContext) []RetrievedChunk {
	if len(chunks) == 0 {
		return chunks
	}

	// 将 RetrievedChunk 转换为 aiclient.RetrievedChunk（rerank 接口要求）
	candidates := make([]aiclient.RetrievedChunk, len(chunks))
	for i, c := range chunks {
		candidates[i] = aiclient.RetrievedChunk{
			ID:    c.ID,
			Text:  c.Content,
			Score: c.Score,
		}
	}

	reranked, err := p.rerank.Rerank(context.Background(), sc.Question, candidates, sc.TopK)
	if err != nil {
		// rerank 失败时降级：按原始分数排序后截取 TopK
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].Score > chunks[j].Score })
		if sc.TopK > 0 && len(chunks) > sc.TopK {
			return chunks[:sc.TopK]
		}
		return chunks
	}

	// 将 rerank 结果映射回 RetrievedChunk（更新 Score，保留原始字段）
	idToChunk := make(map[string]RetrievedChunk, len(chunks))
	for _, c := range chunks {
		idToChunk[c.ID] = c
	}

	result := make([]RetrievedChunk, 0, len(reranked))
	for _, r := range reranked {
		if c, ok := idToChunk[r.ID]; ok {
			c.Score = r.Score
			result = append(result, c)
		}
	}
	return result
}
