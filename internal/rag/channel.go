// Package rag 包含检索增强生成（RAG）相关的核心业务逻辑。
//
// 本文件定义 RAG 检索阶段使用的“检索通道”。检索通道可以理解为不同的查资料策略：
// 有些通道会根据意图识别结果，只去最相关的知识库集合中查；有些通道会在意图不够明确时，
// 到指定知识库范围内做全局向量检索，作为兜底方案。
//
// 通俗来说：这个文件决定“用户问题应该去哪些知识库集合里查、怎么并行查、查完后如何
// 标记结果来源和通道置信度”。它不直接负责回答问题，而是为后续 RAG 组装答案准备候选
// 文档片段。
package rag

import (
	"context"
	"fmt"
	"sync"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/internal/knowledge"
)

// IntentDirectedChannel 是基于意图识别结果的定向检索通道。
//
// 当用户问题被分类到一个或多个高置信度知识库意图时，该通道只检索这些意图绑定的
// Milvus collection。相比全局检索，它的搜索范围更小，通常结果更精准、噪声更少。
type IntentDirectedChannel struct {
	retriever *MilvusRetriever
	config    config.IntentDirectedChannelConfig
}

// NewIntentDirectedChannel 创建基于意图的定向检索通道。
//
// retriever 负责实际访问 Milvus；cfg 控制最小意图分数、每个意图的 TopK 放大倍数等
// 通道级策略参数。
func NewIntentDirectedChannel(retriever *MilvusRetriever, cfg config.IntentDirectedChannelConfig) *IntentDirectedChannel {
	return &IntentDirectedChannel{retriever: retriever, config: cfg}
}

// Name 返回通道名称，用于日志、调试和结果归因。
func (c *IntentDirectedChannel) Name() string { return "intent_directed" }

// Priority 返回通道优先级，数值越小优先级越高。
func (c *IntentDirectedChannel) Priority() int { return 1 }

// IsEnabled 判断当前搜索上下文是否应该启用定向检索。
//
// 只要存在一个知识库意图的分数达到最小阈值，就说明问题已经有明确方向，可以优先走
// 意图定向检索。
func (c *IntentDirectedChannel) IsEnabled(sc SearchContext) bool {
	for _, intent := range sc.IntentGroup.KbIntents {
		if intent.Score >= c.config.MinIntentScore {
			return true
		}
	}
	return false
}

// Search 按高置信度意图并行检索对应的知识库集合。
//
// 每个达标意图会映射到一个 Milvus collection，并独立发起向量检索。单个 collection
// 检索失败不会中断整体流程，避免一个知识库异常影响其它知识库的候选结果。返回结果中会
// 补充 KbID，并用参与检索的意图平均分作为该通道置信度。
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

// VectorGlobalChannel 是全局向量检索通道，用于意图不明确时的兜底检索。
//
// 当没有系统意图、也没有足够高置信度的知识库意图时，该通道会在本次请求指定的知识库
// 集合中并行检索，尽量保证用户问题仍然能拿到候选文档。
type VectorGlobalChannel struct {
	retriever *MilvusRetriever
	kbRepo    knowledge.KBRepo
	config    config.VectorGlobalChannelConfig
}

// NewVectorGlobalChannel 创建全局向量检索通道。
//
// kbRepo 用于把知识库 ID 解析为 Milvus collection 名称，retriever 负责执行实际的
// 向量检索，cfg 控制启用阈值和 TopK 放大策略。
func NewVectorGlobalChannel(retriever *MilvusRetriever, kbRepo knowledge.KBRepo, cfg config.VectorGlobalChannelConfig) *VectorGlobalChannel {
	return &VectorGlobalChannel{retriever: retriever, kbRepo: kbRepo, config: cfg}
}

// Name 返回通道名称，用于日志、调试和结果归因。
func (c *VectorGlobalChannel) Name() string { return "vector_global" }

// Priority 返回通道优先级，数值越小优先级越高。
func (c *VectorGlobalChannel) Priority() int { return 10 }

// IsEnabled 判断当前搜索上下文是否应该启用全局向量检索。
//
// 纯系统问题（AllSystemOnly）不需要查知识库，直接禁用；
// 已存在高置信度 KB 意图时交给 IntentDirectedChannel，避免重复扩大检索范围；
// 混合 SYSTEM+KB 场景 AllSystemOnly=false，仍按 KB 候选打分判定是否兜底。
func (c *VectorGlobalChannel) IsEnabled(sc SearchContext) bool {
	if sc.IntentGroup.AllSystemOnly {
		return false
	}
	for _, intent := range sc.IntentGroup.KbIntents {
		if intent.Score >= c.config.ConfidenceThreshold {
			return false // 有高置信度 KB 意图，由 IntentDirectedChannel 处理
		}
	}
	return true
}

// Search 并行检索本次请求指定的所有知识库集合，并合并候选片段。
//
// 该通道会先根据 KbIDs 加载 collection 名称，再对每个 collection 并行发起向量检索。
// 单个 collection 检索失败会被跳过，不影响其它集合的结果。全局检索使用固定置信度，
// 与 Java 版本策略保持一致。
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

// loadCollections 根据知识库 ID 加载 Milvus collection 与知识库 ID 的映射关系。
//
// 返回的两个 map 当前内容一致，分别服务于“需要检索哪些 collection”和“检索结果应该
// 回填哪个 KbID”两个语义，便于后续扩展不同来源的 collection 映射策略。
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
