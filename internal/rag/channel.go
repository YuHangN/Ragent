package rag

import (
	"context"
	"fmt"
	"sync"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/internal/knowledge"
)

type IntentDirectedChannel struct {
	retriever *MilvusRetriever
	config    config.IntentDirectedChannelConfig
}

func NewIntentDirectedChannel(retriever *MilvusRetriever, cfg config.IntentDirectedChannelConfig) *IntentDirectedChannel {
	return &IntentDirectedChannel{retriever: retriever, config: cfg}
}

func (c *IntentDirectedChannel) Name() string  { return "intent_directed" }
func (c *IntentDirectedChannel) Priority() int { return 1 }

func (c *IntentDirectedChannel) IsEnabled(sc SearchContext) bool {
	for _, intent := range sc.IntentGroup.KbIntents {
		if intent.Score >= c.config.MinIntentScore {
			return true
		}
	}
	return false
}

func (c *IntentDirectedChannel) Search(ctx context.Context, sc SearchContext) (SearchChannelResult, error) {
	// 过滤出分数达标的 KB 意图
	var intents []IntentCandidate
	for _, intent := range sc.IntentGroup.KbIntents {
		if intent.Score >= c.config.MinIntentScore {
			intents = append(intents, intent)
		}
	}

	topKPerIntent := sc.TopK * c.config.TopKMultiplier
	if topKPerIntent <= 0 {
		topKPerIntent = sc.TopK * 2
	}

	// 并行检索每个意图对应的集合
	type searchRes struct {
		chunks []RetrievedChunk
		err    error
	}
	resCh := make(chan searchRes, len(intents))
	var wg sync.WaitGroup

	for _, intent := range intents {
		intent := intent
		wg.Add(1)
		go func() {
			defer wg.Done()
			chunks, err := c.retriever.Search(ctx, intent.CollectionName, sc.Question, topKPerIntent)
			if err != nil {
				resCh <- searchRes{err: err}
				return
			}
			// 将 KbID 注入 chunk（Milvus 只存 doc_id，kb_id 从 intent 补充）
			for i := range chunks {
				chunks[i].KbID = intent.KbID
			}
			resCh <- searchRes{chunks: chunks}
		}()
	}

	wg.Wait()
	close(resCh)

	var all []RetrievedChunk
	for res := range resCh {
		if res.err != nil {
			continue // 单个通道失败不阻断整体
		}
		all = append(all, res.chunks...)
	}

	// 计算平均置信度作为通道置信度
	var avgScore float64
	for _, intent := range intents {
		avgScore += intent.Score
	}
	if len(intents) > 0 {
		avgScore /= float64(len(intents))
	}

	return SearchChannelResult{
		ChannelName: c.Name(),
		Priority:    c.Priority(),
		Chunks:      all,
		Confidence:  avgScore,
	}, nil
}

// ──── VectorGlobalChannel ─────────────────────────────────

// VectorGlobalChannel 在所有 KB 集合中执行全局向量检索，作为兜底策略。
type VectorGlobalChannel struct {
	retriever *MilvusRetriever
	kbRepo    knowledge.KBRepo
	config    config.VectorGlobalChannelConfig
}

func NewVectorGlobalChannel(retriever *MilvusRetriever, kbRepo knowledge.KBRepo, cfg config.VectorGlobalChannelConfig) *VectorGlobalChannel {
	return &VectorGlobalChannel{retriever: retriever, kbRepo: kbRepo, config: cfg}
}

func (c *VectorGlobalChannel) Name() string  { return "vector_global" }
func (c *VectorGlobalChannel) Priority() int { return 10 }

func (c *VectorGlobalChannel) IsEnabled(sc SearchContext) bool {
	if sc.IntentGroup.HasSystem {
		return false
	}
	for _, intent := range sc.IntentGroup.KbIntents {
		if intent.Score >= c.config.ConfidenceThreshold {
			return false // 有高置信度 KB 意图，由 IntentDirectedChannel 处理
		}
	}
	return true
}

// Search 并行检索所有指定 KB 的集合，合并结果。
func (c *VectorGlobalChannel) Search(ctx context.Context, sc SearchContext) (SearchChannelResult, error) {
	// 加载要搜索的 KB 列表（当前只取指定 ID，后续可扩展为全部 KB）
	collections, kbMap, err := c.loadCollections(sc.KbIDs)
	if err != nil {
		return SearchChannelResult{}, fmt.Errorf("vector_global: load collections: %w", err)
	}

	topKPerCol := sc.TopK * c.config.TopKMultiplier
	if topKPerCol <= 0 {
		topKPerCol = sc.TopK * 3
	}

	type searchRes struct {
		chunks []RetrievedChunk
		kbID   int64
		err    error
	}
	resCh := make(chan searchRes, len(collections))
	var wg sync.WaitGroup

	for colName, kbID := range collections {
		colName, kbID := colName, kbID
		wg.Add(1)
		go func() {
			defer wg.Done()
			chunks, err := c.retriever.Search(ctx, colName, sc.Question, topKPerCol)
			resCh <- searchRes{chunks: chunks, kbID: kbID, err: err}
		}()
	}

	wg.Wait()
	close(resCh)

	var all []RetrievedChunk
	for res := range resCh {
		if res.err != nil {
			continue
		}
		for i := range res.chunks {
			res.chunks[i].KbID = kbMap[res.chunks[i].CollectionName]
		}
		all = append(all, res.chunks...)
	}

	return SearchChannelResult{
		ChannelName: c.Name(),
		Priority:    c.Priority(),
		Chunks:      all,
		Confidence:  0.7, // 全局检索固定置信度，与 Java 一致
	}, nil
}

// loadCollections 根据 kbIDs 加载 collectionName → kbID 映射。
func (c *VectorGlobalChannel) loadCollections(kbIDs []int64) (map[string]int64, map[string]int64, error) {
	cols := make(map[string]int64)
	kbMap := make(map[string]int64)
	for _, id := range kbIDs {
		kb, err := c.kbRepo.FindByID(id)
		if err != nil {
			continue
		}
		colName := knowledge.BuildCollectionName(kb.ID)
		cols[colName] = kb.ID
		kbMap[colName] = kb.ID
	}
	return cols, kbMap, nil
}
