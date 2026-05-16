package rag

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// MilvusRetriever 封装向量 embed + Milvus 搜索。
type MilvusRetriever struct {
	milvus    milvusclient.Client
	embedding aiclient.EmbeddingService
}

func NewMilvusRetriever(milvus milvusclient.Client, embedding aiclient.EmbeddingService) *MilvusRetriever {
	return &MilvusRetriever{milvus: milvus, embedding: embedding}
}

// Search 对单个集合执行向量搜索，返回 topK 条结果。
//
// partitions 为 nil 或空时，Milvus 会扫描该 collection 下所有 partition（VectorGlobal
// 兜底场景）。传入具体 partition 名时只在该 partition 内检索（IntentDirected 精准场景）。
func (r *MilvusRetriever) Search(ctx context.Context, collectionName string, partitions []string, query string, topK int) ([]RetrievedChunk, error) {
	// 1. 将查询文本转为向量
	vecs, err := r.embedding.EmbedBatch(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("retriever: embed query: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("retriever: empty embedding returned")
	}

	queryVec := entity.FloatVector(vecs[0])

	// 2. 构建 Milvus 搜索参数（IVF_FLAT 索引）
	sp, err := entity.NewIndexIvfFlatSearchParam(16)
	if err != nil {
		return nil, fmt.Errorf("retriever: build search params: %w", err)
	}

	// 3. 执行向量搜索，返回 id / doc_id / content 字段
	results, err := r.milvus.Search(
		ctx,
		collectionName,
		partitions,                          // 空 = 该 collection 下所有 partition
		"",                                  // expr 过滤（空=不过滤）
		[]string{"id", "doc_id", "content"}, // 返回字段
		[]entity.Vector{queryVec},
		"embedding", // 向量字段名，与 Phase 5 indexer 写入字段名一致
		entity.COSINE,
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("retriever: milvus search on %q: %w", collectionName, err)
	}

	return parseSearchResults(results, collectionName), nil
}

// parseSearchResults 将 Milvus SearchResult 转换为 []RetrievedChunk。
func parseSearchResults(results []milvusclient.SearchResult, collectionName string) []RetrievedChunk {
	if len(results) == 0 {
		return nil
	}
	result := results[0]
	chunks := make([]RetrievedChunk, 0, result.ResultCount)

	idCol := result.Fields.GetColumn("id")
	docIDCol := result.Fields.GetColumn("doc_id")
	contentCol := result.Fields.GetColumn("content")

	for i := 0; i < result.ResultCount; i++ {
		chunk := RetrievedChunk{
			Score:          result.Scores[i],
			CollectionName: collectionName,
		}
		if idCol != nil {
			if v, err := idCol.GetAsString(i); err == nil {
				chunk.ID = v
			}
		}
		if docIDCol != nil {
			if v, err := docIDCol.GetAsInt64(i); err == nil {
				chunk.DocID = v
			}
		}
		if contentCol != nil {
			if v, err := contentCol.GetAsString(i); err == nil {
				chunk.Content = v
			}
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}
