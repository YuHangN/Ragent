package ingestion

import (
	"context"
	"fmt"

	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type IndexerNode struct {
	milvus milvusclient.Client
}

func NewIndexerNode(milvus milvusclient.Client) *IndexerNode {
	return &IndexerNode{milvus: milvus}
}

func (n *IndexerNode) Name() string { return "indexer" }

func (n *IndexerNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if len(ic.Chunks) == 0 {
		return Fail(fmt.Errorf("indexer: no chunks to index"))
	}
	if ic.KBCollectionName == "" {
		return Fail(fmt.Errorf("indexer: KBCollectionName is empty"))
	}

	// Validate that all embeddings are present.
	dim := 0
	for i, ch := range ic.Chunks {
		if len(ch.Embedding) == 0 {
			return Fail(fmt.Errorf("indexer: chunk %d has no embedding", i))
		}
		if dim == 0 {
			dim = len(ch.Embedding)
		}
	}

	// Build columns.
	ids := make([]string, len(ic.Chunks))
	docIDs := make([]int64, len(ic.Chunks))
	chunkIndexes := make([]int32, len(ic.Chunks))
	contents := make([]string, len(ic.Chunks))
	embeddings := make([][]float32, len(ic.Chunks))

	for i, ch := range ic.Chunks {
		ids[i] = ch.ChunkID
		docIDs[i] = ic.DocID
		chunkIndexes[i] = int32(ch.Index)
		content := ch.Content
		if len([]rune(content)) > 65535 {
			content = string([]rune(content)[:65535])
		}
		contents[i] = content
		embeddings[i] = ch.Embedding
	}

	cols := []entity.Column{
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnInt64("doc_id", docIDs),
		entity.NewColumnInt32("chunk_index", chunkIndexes),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnFloatVector("embedding", dim, embeddings),
	}

	// 空 PartitionName 让 Milvus 走 _default 系统分区，与未启用 Phase 6.7 时行为一致。
	_, err := n.milvus.Insert(ctx, ic.KBCollectionName, ic.PartitionName, cols...)
	if err != nil {
		return Fail(fmt.Errorf("indexer: Milvus insert into %s/%s: %w", ic.KBCollectionName, ic.PartitionName, err))
	}

	partLabel := ic.PartitionName
	if partLabel == "" {
		partLabel = "_default"
	}
	return OK(fmt.Sprintf("indexed %d chunks into collection %s partition %s", len(ic.Chunks), ic.KBCollectionName, partLabel))
}
