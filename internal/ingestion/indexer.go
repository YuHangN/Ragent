package ingestion

import (
	"context"
	"fmt"

	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"go.uber.org/zap"
)

// IndexerParams 控制 IndexerNode 写入行为。
type IndexerParams struct {
	// AutoCreatePartition：写入前对缺失的 partition 调用 CreatePartition。
	// 关闭时缺失即 fail-fast，防止数据写错位置无人察觉。
	AutoCreatePartition bool
}

// IndexerNode 把 ic.Chunks 按 chunk.TargetPartition 分组后写入 Milvus。
// 空 TargetPartition 回退 ic.PartitionName（doc 级 fallback）。
type IndexerNode struct {
	milvus milvusclient.Client
	params IndexerParams
}

func NewIndexerNode(milvus milvusclient.Client, params IndexerParams) *IndexerNode {
	return &IndexerNode{milvus: milvus, params: params}
}

func (n *IndexerNode) Name() string { return "indexer" }

func (n *IndexerNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if len(ic.Chunks) == 0 {
		return Fail(fmt.Errorf("indexer: no chunks to index"))
	}
	if ic.KBCollectionName == "" {
		return Fail(fmt.Errorf("indexer: KBCollectionName is empty"))
	}

	dim := 0
	for i, ch := range ic.Chunks {
		if len(ch.Embedding) == 0 {
			return Fail(fmt.Errorf("indexer: chunk %d has no embedding", i))
		}
		if dim == 0 {
			dim = len(ch.Embedding)
		}
	}

	// 按 chunk.TargetPartition 分组；空值回退 ic.PartitionName。
	groups := make(map[string][]int)
	for i, ch := range ic.Chunks {
		part := ch.TargetPartition
		if part == "" {
			part = ic.PartitionName
		}
		groups[part] = append(groups[part], i)
	}

	if err := n.ensurePartitions(ctx, ic.KBCollectionName, groups); err != nil {
		return Fail(err)
	}

	totalInserted := 0
	for part, idxs := range groups {
		cols := buildInsertColumns(ic, idxs, dim)
		if _, err := n.milvus.Insert(ctx, ic.KBCollectionName, part, cols...); err != nil {
			return Fail(fmt.Errorf("indexer: Milvus insert %s/%s: %w", ic.KBCollectionName, part, err))
		}
		totalInserted += len(idxs)
	}

	return OK(fmt.Sprintf("indexer: %d chunks into %d partitions of %s",
		totalInserted, len(groups), ic.KBCollectionName))
}

// ensurePartitions 检查每个分组 partition 是否存在；不存在时按配置决定 Create 还是 fail。
// 空字符串 partition（→ _default 系统分区）不需要检查。
func (n *IndexerNode) ensurePartitions(ctx context.Context, coll string, groups map[string][]int) error {
	for part := range groups {
		if part == "" {
			continue // _default 系统分区永远存在
		}
		exists, err := n.milvus.HasPartition(ctx, coll, part)
		if err != nil {
			return fmt.Errorf("indexer: HasPartition %s/%s: %w", coll, part, err)
		}
		if exists {
			continue
		}
		if !n.params.AutoCreatePartition {
			return fmt.Errorf("indexer: partition %s/%s missing (AutoCreatePartition=false)", coll, part)
		}
		if err := n.milvus.CreatePartition(ctx, coll, part); err != nil {
			return fmt.Errorf("indexer: CreatePartition %s/%s: %w", coll, part, err)
		}
		zap.L().Info("indexer: auto-created partition",
			zap.String("collection", coll), zap.String("partition", part))
	}
	return nil
}

func buildInsertColumns(ic *IngestionContext, idxs []int, dim int) []entity.Column {
	ids := make([]string, len(idxs))
	docIDs := make([]int64, len(idxs))
	chunkIndexes := make([]int32, len(idxs))
	contents := make([]string, len(idxs))
	embeddings := make([][]float32, len(idxs))

	for k, i := range idxs {
		ch := ic.Chunks[i]
		ids[k] = ch.ChunkID
		docIDs[k] = ic.DocID
		chunkIndexes[k] = int32(ch.Index)
		content := ch.Content
		if len([]rune(content)) > 65535 {
			content = string([]rune(content)[:65535])
		}
		contents[k] = content
		embeddings[k] = ch.Embedding
	}

	return []entity.Column{
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnInt64("doc_id", docIDs),
		entity.NewColumnInt32("chunk_index", chunkIndexes),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnFloatVector("embedding", dim, embeddings),
	}
}
