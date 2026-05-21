package ingestion

import (
	"context"
	"fmt"
	"time"

	"github.com/YuHangN/ragent-go/internal/intent"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// ChunkRouterParams 控制 ChunkRouterNode 的运行时行为，方便测试与配置注入解耦。
type ChunkRouterParams struct {
	MinScore    float64
	Concurrency int // batch 间并发数
	BatchSize   int // 单 batch 包含的 chunk 数
	MaxRetries  int // LLM 调用失败的最大重试次数
}

// ChunkRouterNode 对每个 chunk 调 intent.Classifier 决定目标 Milvus partition。
//
// 流程（标配 A）：
//   - 按 BatchSize 切 chunk 成多个 batch；
//   - batch 之间用 errgroup.SetLimit(Concurrency) 并发；
//   - 每个 batch 调一次 LLM；失败时按指数退避重试 MaxRetries 次；
//   - 重试用尽 / 低分 / 非 KB 类型 → 对应 chunk 走 docPartition，不写 routing metadata；
//   - 成功的 chunk 把路由结果写入 chunk.Metadata["routing"]。
//
// 节点本身永远 Success——单 batch 失败只走 fallback，不阻断后续 pipeline。
// fallback chunk 留在 docPartition 里，由 query 侧 VectorGlobalChannel 兜底检索捞回。
type ChunkRouterNode struct {
	classifier   *intent.Classifier
	kbID         int64
	docPartition string
	params       ChunkRouterParams
}

func NewChunkRouterNode(classifier *intent.Classifier, kbID int64, docPartition string, params ChunkRouterParams) *ChunkRouterNode {
	if params.Concurrency <= 0 {
		params.Concurrency = 4
	}
	if params.BatchSize <= 0 {
		params.BatchSize = 8
	}
	if params.MaxRetries < 0 {
		params.MaxRetries = 0
	}
	return &ChunkRouterNode{
		classifier:   classifier,
		kbID:         kbID,
		docPartition: docPartition,
		params:       params,
	}
}

func (n *ChunkRouterNode) Name() string { return "chunk_router" }

func (n *ChunkRouterNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if len(ic.Chunks) == 0 {
		return OK("chunk_router: no chunks")
	}
	if n.classifier == nil {
		zap.L().Warn("chunk_router: classifier nil, fallback all")
		for i := range ic.Chunks {
			n.applyFallback(&ic.Chunks[i])
		}
		return OK("chunk_router: classifier nil → all fallback")
	}

	batches := splitBatches(len(ic.Chunks), n.params.BatchSize)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(n.params.Concurrency)

	for _, b := range batches {
		b := b
		g.Go(func() error {
			n.routeBatch(gctx, ic, b)
			return nil // 单 batch 失败已在内部处理为 fallback
		})
	}
	_ = g.Wait()

	return OK(fmt.Sprintf("chunk_router: routed %d chunks in %d batches", len(ic.Chunks), len(batches)))
}

func (n *ChunkRouterNode) routeBatch(ctx context.Context, ic *IngestionContext, idxs []int) {
	contents := make([]string, len(idxs))
	for i, ci := range idxs {
		contents[i] = ic.Chunks[ci].Content
	}

	results, err := n.classifyWithRetry(ctx, contents)
	if err != nil {
		zap.L().Warn("chunk_router: batch classify exhausted retries",
			zap.Int("batch_size", len(idxs)),
			zap.Error(err))
		for _, ci := range idxs {
			n.applyFallback(&ic.Chunks[ci])
		}
		return
	}

	for i, ci := range idxs {
		ch := &ic.Chunks[ci]
		if i >= len(results) || len(results[i]) == 0 {
			n.applyFallback(ch)
			continue
		}
		top := results[i][0]
		if top.Kind != intent.KindKB || top.PartitionName == "" {
			n.applyFallback(ch)
			continue
		}
		ch.TargetPartition = top.PartitionName
		n.writeRoutingMetadata(ch, top)
	}
}

// classifyWithRetry 在 ClassifyChunks 失败时做指数退避重试。
// MaxRetries=2 表示首次调用 + 2 次重试 = 共最多 3 次。
func (n *ChunkRouterNode) classifyWithRetry(ctx context.Context, contents []string) ([][]intent.Candidate, error) {
	var lastErr error
	attempts := n.params.MaxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		results, err := n.classifier.ClassifyChunks(ctx, n.kbID, contents, 1, n.params.MinScore)
		if err == nil {
			return results, nil
		}
		lastErr = err
		if attempt < attempts-1 {
			backoff := time.Duration(200<<attempt) * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, lastErr
}

// applyFallback 把 chunk 路由到默认 partition，不写 routing metadata。
// 没有产生 intent 决策，所以也不留 routing 痕迹；query 侧 VectorGlobal 兜底会扫到。
func (n *ChunkRouterNode) applyFallback(ch *VectorChunk) {
	ch.TargetPartition = n.docPartition
}

func (n *ChunkRouterNode) writeRoutingMetadata(ch *VectorChunk, c intent.Candidate) {
	if ch.Metadata == nil {
		ch.Metadata = make(map[string]any)
	}
	ch.Metadata["routing"] = map[string]any{
		"node_id":    c.NodeID,
		"node_name":  c.NodeName,
		"partition":  c.PartitionName,
		"score":      c.Score,
		"decided_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func splitBatches(total, size int) [][]int {
	if size <= 0 {
		size = total
	}
	var out [][]int
	for start := 0; start < total; start += size {
		end := start + size
		if end > total {
			end = total
		}
		batch := make([]int, 0, end-start)
		for i := start; i < end; i++ {
			batch = append(batch, i)
		}
		out = append(out, batch)
	}
	return out
}
