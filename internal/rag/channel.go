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

// directedTask 描述一次定向检索任务：用某个子问题文本，去查某个 KB 意图的目标集合。
//
// 把"哪个子问题 × 哪个意图"显式表达成任务，channel 才能保留 sub-question 精度。
// 同一个 sub-question 命中多个 KB 意图会展开成多条任务；不同 sub-question 命中
// 同一个意图集合也展开成多条任务（查询文本不同），后续按 chunk.ID 去重即可。
type directedTask struct {
	subQuestion string
	intent      IntentCandidate
}

// IsEnabled 判断当前搜索上下文是否应该启用定向检索。
//
// 只要任意子问题的任意 KB 候选分数达到最小阈值，就说明问题已经有明确方向，可以优先走
// 意图定向检索。SubIntents 为空（resolver 未生效）时退回旧逻辑，看扁平的 IntentGroup。
func (c *IntentDirectedChannel) IsEnabled(sc SearchContext) bool {
	if len(sc.SubIntents) > 0 {
		for _, sq := range sc.SubIntents {
			for _, intent := range sq.Candidates {
				if intent.Kind == IntentKindKB && intent.Score >= c.config.MinIntentScore {
					return true
				}
			}
		}
		return false
	}
	for _, intent := range sc.IntentGroup.KbIntents {
		if intent.Score >= c.config.MinIntentScore {
			return true
		}
	}
	return false
}

// Search 按 (子问题, KB 意图) 对并行检索对应的知识库集合。
//
// 用子问题文本而非主问题去查目标集合，避免多主题主问题污染单主题集合的检索精度
// （findings.md "channel 检索丢弃子问题精度"）。单个任务失败不阻断其它任务；返回
// 结果会补全 KbID，并以所有参与检索任务的意图平均分作为通道置信度。
//
// 当 SubIntents 为空（resolver 未生效或 KbIDs 空）时退回旧行为：用主问题查所有
// 高分 KB 意图集合，保证向后兼容。
func (c *IntentDirectedChannel) Search(ctx context.Context, sc SearchContext) (SearchChannelResult, error) {
	tasks := c.buildTasks(sc)

	topKPerIntent := sc.TopK * c.config.TopKMultiplier
	if topKPerIntent <= 0 {
		topKPerIntent = sc.TopK * 2
	}

	type searchRes struct {
		chunks []RetrievedChunk
		err    error
	}
	resCh := make(chan searchRes, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Phase 6 临时回退：IntentNode.CollectionName 为空时退回 KB 默认集合，
			// 避免运维填了"假"集合名时整通道查空。Phase 6.7 会把字段语义切到 partition，
			// 届时这里改成 collection 固定 + partition 由 intent 决定（findings.md 详述）。
			col := task.intent.CollectionName
			if col == "" {
				col = knowledge.BuildCollectionName(task.intent.KbID)
			}
			chunks, err := c.retriever.Search(ctx, col, task.subQuestion, topKPerIntent)
			if err != nil {
				resCh <- searchRes{err: err}
				return
			}
			// 将 KbID 注入 chunk（Milvus 只存 doc_id，kb_id 从 intent 补充）
			for i := range chunks {
				chunks[i].KbID = task.intent.KbID
			}
			resCh <- searchRes{chunks: chunks}
		}()
	}

	wg.Wait()
	close(resCh)

	var all []RetrievedChunk
	for res := range resCh {
		if res.err != nil {
			continue // 单个任务失败不阻断整体
		}
		all = append(all, res.chunks...)
	}

	// 用所有任务的意图平均分作为通道置信度（rerank 会做最终排序，这里粗算即可）
	var avgScore float64
	for _, t := range tasks {
		avgScore += t.intent.Score
	}
	if len(tasks) > 0 {
		avgScore /= float64(len(tasks))
	}

	return SearchChannelResult{
		ChannelName: c.Name(),
		Priority:    c.Priority(),
		Chunks:      all,
		Confidence:  avgScore,
	}, nil
}

// buildTasks 把 SearchContext 展开成具体的检索任务列表。
//
// 优先消费 SubIntents（per-sub-question 路由）；SubIntents 为空时退回旧行为，
// 用主问题查所有高分 KB 意图集合。
func (c *IntentDirectedChannel) buildTasks(sc SearchContext) []directedTask {
	if len(sc.SubIntents) == 0 {
		var tasks []directedTask
		for _, intent := range sc.IntentGroup.KbIntents {
			if intent.Score >= c.config.MinIntentScore {
				tasks = append(tasks, directedTask{subQuestion: sc.Question, intent: intent})
			}
		}
		return tasks
	}

	var tasks []directedTask
	for _, sq := range sc.SubIntents {
		for _, intent := range sq.Candidates {
			if intent.Kind != IntentKindKB {
				continue // SYSTEM / MCP 不归本通道
			}
			if intent.Score < c.config.MinIntentScore {
				continue
			}
			tasks = append(tasks, directedTask{subQuestion: sq.SubQuestion, intent: intent})
		}
	}
	return tasks
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

// globalTask 描述一次兜底检索任务：用某个查询文本，去某个 KB 默认集合查。
//
// 当某个子问题没有任何高置信度 KB 意图时，由本通道兜底；查询文本就用该子问题本身，
// 而不是合并后的主问题，保持 sub-question 精度。
type globalTask struct {
	query        string
	collection   string
	kbID         int64
}

// IsEnabled 判断当前搜索上下文是否应该启用全局向量检索。
//
// 启用条件：
//   - 不是纯 SYSTEM 问题（AllSystemOnly=false）
//   - 至少存在一个"未覆盖"子问题，即没有任何 KB 候选分数 ≥ ConfidenceThreshold
//   - 或者根本没有 SubIntents（resolver 未生效，整个就当兜底跑）
//
// 这样在"2 高分 + 1 低分"的混合场景下，本通道仍会针对低分子问题独立兜底，
// 不会因为另外两个子问题命中高分意图就把自己整体关闭（findings.md Phase 6.6）。
func (c *VectorGlobalChannel) IsEnabled(sc SearchContext) bool {
	if sc.IntentGroup.AllSystemOnly {
		return false
	}
	if len(sc.SubIntents) == 0 {
		return true // 没有子问题级路由信息，整体兜底
	}
	for _, sq := range sc.SubIntents {
		if !c.isSubQuestionCovered(sq) {
			return true
		}
	}
	return false // 所有子问题都被高置信度意图覆盖
}

// isSubQuestionCovered 判断单个子问题是否已被某个高置信度 KB 意图覆盖。
func (c *VectorGlobalChannel) isSubQuestionCovered(sq SubQuestionIntent) bool {
	for _, intent := range sq.Candidates {
		if intent.Kind == IntentKindKB && intent.Score >= c.config.ConfidenceThreshold {
			return true
		}
	}
	return false
}

// Search 对每个未覆盖子问题在所有指定 KB 默认集合中并行检索，并合并候选片段。
//
// 与旧实现的关键差异：用未覆盖子问题文本去查，而不是用合并后的主问题。这样混合
// 意图场景下，本通道只补低分子问题的"覆盖盲区"，而不是再用主问题对所有集合做一遍
// 全量召回。SubIntents 为空时退回旧行为（用主问题查所有指定 KB）。
//
// 单个任务失败被跳过，不阻断其它任务。
func (c *VectorGlobalChannel) Search(ctx context.Context, sc SearchContext) (SearchChannelResult, error) {
	collections, kbMap, err := c.loadCollections(sc.KbIDs)
	if err != nil {
		return SearchChannelResult{}, fmt.Errorf("vector_global: load collections: %w", err)
	}

	tasks := c.buildTasks(sc, collections)

	topKPerCol := sc.TopK * c.config.TopKMultiplier
	if topKPerCol <= 0 {
		topKPerCol = sc.TopK * 3
	}

	type searchRes struct {
		chunks []RetrievedChunk
		kbID   int64
		err    error
	}
	resCh := make(chan searchRes, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			chunks, err := c.retriever.Search(ctx, task.collection, task.query, topKPerCol)
			resCh <- searchRes{chunks: chunks, kbID: task.kbID, err: err}
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

// buildTasks 把"未覆盖子问题 × 指定 KB 默认集合"展开成 cartesian 任务列表。
//
// SubIntents 为空时退回主问题；混合意图场景下只为未覆盖的子问题分配兜底任务。
func (c *VectorGlobalChannel) buildTasks(sc SearchContext, collections map[string]int64) []globalTask {
	queries := c.uncoveredQueries(sc)
	if len(queries) == 0 || len(collections) == 0 {
		return nil
	}
	tasks := make([]globalTask, 0, len(queries)*len(collections))
	for _, q := range queries {
		for col, kbID := range collections {
			tasks = append(tasks, globalTask{query: q, collection: col, kbID: kbID})
		}
	}
	return tasks
}

// uncoveredQueries 返回需要本通道兜底的查询文本列表。
//
// SubIntents 为空 → 用主问题（resolver 未生效，整体兜底）。
// 否则只挑出未被高置信度 KB 意图覆盖的子问题；如果 SubIntents 全被覆盖，IsEnabled
// 已经把通道关掉了，理论上不会走到这里。
func (c *VectorGlobalChannel) uncoveredQueries(sc SearchContext) []string {
	if len(sc.SubIntents) == 0 {
		if sc.Question == "" {
			return nil
		}
		return []string{sc.Question}
	}
	var queries []string
	for _, sq := range sc.SubIntents {
		if !c.isSubQuestionCovered(sq) {
			queries = append(queries, sq.SubQuestion)
		}
	}
	return queries
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
