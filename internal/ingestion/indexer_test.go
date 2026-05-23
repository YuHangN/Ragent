package ingestion

import (
	"context"
	"errors"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubMilvus 实现 IndexerNode 用到的 client 子集，其它方法嵌入 client.Client 走 nil-panic。
type stubMilvus struct {
	client.Client
	calls         []insertCall
	existingParts map[string]map[string]bool // coll → partition → exists
	createCalls   []createCall
	createErr     error
}

type insertCall struct {
	collection string
	partition  string
	rowCount   int
}

type createCall struct {
	collection string
	partition  string
}

func newStubMilvus(existing map[string]map[string]bool) *stubMilvus {
	if existing == nil {
		existing = map[string]map[string]bool{}
	}
	return &stubMilvus{existingParts: existing}
}

func (m *stubMilvus) HasPartition(_ context.Context, coll, part string) (bool, error) {
	if m.existingParts[coll] == nil {
		return false, nil
	}
	return m.existingParts[coll][part], nil
}

func (m *stubMilvus) CreatePartition(_ context.Context, coll, part string, _ ...client.CreatePartitionOption) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.existingParts[coll] == nil {
		m.existingParts[coll] = map[string]bool{}
	}
	m.existingParts[coll][part] = true
	m.createCalls = append(m.createCalls, createCall{collection: coll, partition: part})
	return nil
}

func (m *stubMilvus) Insert(_ context.Context, coll, part string, cols ...entity.Column) (entity.Column, error) {
	rowCount := 0
	if len(cols) > 0 {
		rowCount = cols[0].Len()
	}
	m.calls = append(m.calls, insertCall{collection: coll, partition: part, rowCount: rowCount})
	return entity.NewColumnInt64("id", nil), nil
}

func newChunk(idx int, content, targetPart string, dim int) VectorChunk {
	emb := make([]float32, dim)
	for i := range emb {
		emb[i] = float32(idx + 1)
	}
	return VectorChunk{
		ChunkID:         "id-" + content,
		Index:           idx,
		Content:         content,
		TargetPartition: targetPart,
		Embedding:       emb,
	}
}

// TestIndexer_GroupsChunksByPartition 验证：一次写入若 chunk 的 TargetPartition 不同，
// IndexerNode 会按 partition 分组成多次 Milvus.Insert（每组一次），而不是把全部 chunk
// 塞进 docPartition。空 TargetPartition 视作 fallback，归到 ic.PartitionName。
//
// 场景：4 chunks → refund×2 / install×1 / _fallback×1（最后一条空 TargetPartition），
// 应该看到 3 次 Insert，行数分别 2 / 1 / 1。
func TestIndexer_GroupsChunksByPartition(t *testing.T) {
	m := newStubMilvus(map[string]map[string]bool{
		"kb_100": {"refund": true, "install": true, "_fallback": true},
	})
	node := NewIndexerNode(m, IndexerParams{AutoCreatePartition: false})

	ic := &IngestionContext{
		DocID:            1,
		KBCollectionName: "kb_100",
		PartitionName:    "_fallback",
		Chunks: []VectorChunk{
			newChunk(0, "a", "refund", 4),
			newChunk(1, "b", "install", 4),
			newChunk(2, "c", "refund", 4),
			newChunk(3, "d", "", 4), // 空 → 回退 _fallback
		},
	}
	res := node.Execute(context.Background(), ic)
	require.True(t, res.Success, "msg=%s err=%v", res.Message, res.Err)
	require.Len(t, m.calls, 3)
	byPart := map[string]int{}
	for _, c := range m.calls {
		assert.Equal(t, "kb_100", c.collection)
		byPart[c.partition] = c.rowCount
	}
	assert.Equal(t, 2, byPart["refund"])
	assert.Equal(t, 1, byPart["install"])
	assert.Equal(t, 1, byPart["_fallback"])
}

// TestIndexer_AutoCreate_CreatesMissingPartition 验证：AutoCreatePartition=true 时，
// 遇到 Milvus 中尚不存在的 partition，IndexerNode 应先 CreatePartition 再 Insert。
// 这是为了支撑动态意图——新加的 intent.Node 未必有人工预创建对应 partition。
//
// 场景：refund / install 都不在 Milvus，启动 auto-create 后应看到 2 次 Create + 2 次 Insert。
func TestIndexer_AutoCreate_CreatesMissingPartition(t *testing.T) {
	m := newStubMilvus(map[string]map[string]bool{
		"kb_100": {"_fallback": true}, // refund / install 都不存在
	})
	node := NewIndexerNode(m, IndexerParams{AutoCreatePartition: true})

	ic := &IngestionContext{
		DocID:            1,
		KBCollectionName: "kb_100",
		PartitionName:    "_fallback",
		Chunks: []VectorChunk{
			newChunk(0, "a", "refund", 4),
			newChunk(1, "b", "install", 4),
		},
	}
	res := node.Execute(context.Background(), ic)
	require.True(t, res.Success, "msg=%s err=%v", res.Message, res.Err)
	require.Len(t, m.createCalls, 2)
	created := map[string]bool{}
	for _, c := range m.createCalls {
		created[c.partition] = true
	}
	assert.True(t, created["refund"])
	assert.True(t, created["install"])
	require.Len(t, m.calls, 2)
}

// TestIndexer_AutoCreateDisabled_FailsOnMissingPartition 验证：AutoCreatePartition=false
// 时，遇到不存在的 partition 应 fail-fast，**不要**静默退化到 docPartition——避免数据被
// 写到错的 partition 后查不到却没人察觉。生产环境 partition 应由运维显式管理。
func TestIndexer_AutoCreateDisabled_FailsOnMissingPartition(t *testing.T) {
	m := newStubMilvus(map[string]map[string]bool{
		"kb_100": {"_fallback": true},
	})
	node := NewIndexerNode(m, IndexerParams{AutoCreatePartition: false})

	ic := &IngestionContext{
		DocID:            1,
		KBCollectionName: "kb_100",
		PartitionName:    "_fallback",
		Chunks:           []VectorChunk{newChunk(0, "a", "refund", 4)},
	}
	res := node.Execute(context.Background(), ic)
	assert.False(t, res.Success, "AutoCreatePartition=false 时缺失 partition 应 fail")
}

// TestIndexer_AutoCreateError_Propagates 验证：CreatePartition 自身报错时（如 Milvus
// 鉴权拒绝、配额超限），整个节点 fail，不应"创建失败但当作成功 Insert"。
func TestIndexer_AutoCreateError_Propagates(t *testing.T) {
	m := newStubMilvus(map[string]map[string]bool{
		"kb_100": {"_fallback": true},
	})
	m.createErr = errors.New("create denied")
	node := NewIndexerNode(m, IndexerParams{AutoCreatePartition: true})

	ic := &IngestionContext{
		DocID:            1,
		KBCollectionName: "kb_100",
		PartitionName:    "_fallback",
		Chunks:           []VectorChunk{newChunk(0, "a", "refund", 4)},
	}
	res := node.Execute(context.Background(), ic)
	assert.False(t, res.Success)
}

// TestIndexer_AllEmptyTargetUsesDocPartition 验证：所有 chunk 都没有 TargetPartition
// 时（ChunkRouter 未启用、或全部 fallback），统一回退到 ic.PartitionName 单次 Insert，
// 行为等价于路由功能未启用前的老逻辑——确保关掉 ChunkRouter 就能无损降级。
func TestIndexer_AllEmptyTargetUsesDocPartition(t *testing.T) {
	m := newStubMilvus(map[string]map[string]bool{
		"kb_100": {"doc_part": true},
	})
	node := NewIndexerNode(m, IndexerParams{AutoCreatePartition: false})

	ic := &IngestionContext{
		DocID:            1,
		KBCollectionName: "kb_100",
		PartitionName:    "doc_part",
		Chunks: []VectorChunk{
			newChunk(0, "a", "", 4),
			newChunk(1, "b", "", 4),
		},
	}
	res := node.Execute(context.Background(), ic)
	require.True(t, res.Success)
	require.Len(t, m.calls, 1)
	assert.Equal(t, "doc_part", m.calls[0].partition)
	assert.Equal(t, 2, m.calls[0].rowCount)
}
