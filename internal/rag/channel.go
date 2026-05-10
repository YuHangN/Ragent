// Package rag 包含检索增强生成（RAG）的核心逻辑。
//
// 本文件定义“检索通道”。通道只负责召回候选文档片段，不负责生成答案：
//   - IntentDirectedChannel：意图明确时，只查相关知识库。
//   - VectorGlobalChannel：意图不明确时，在用户指定的知识库中兜底查。
//
// 一个请求可能同时启用多个通道。比如“介绍产品 A，也说说售后政策”被拆成两个子问题后，
// 产品问题可以走定向通道，售后问题如果意图分数不够高，还可以走兜底通道。
package rag

import (
	"context"
	"fmt"
	"sync"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/internal/knowledge"
)

// IntentDirectedChannel 根据意图结果做定向检索。
//
// 它适合“已经知道应该查哪个知识库”的场景。比如子问题“产品 A 怎么安装？”
// 命中了“产品 A 手册”意图，分数达到阈值，本通道就只查产品 A 对应的 collection，
// 不会把请求发到所有知识库。
type IntentDirectedChannel struct {
	retriever *MilvusRetriever
	config    config.IntentDirectedChannelConfig
}

// NewIntentDirectedChannel 创建定向检索通道。
//
// retriever 负责真正访问 Milvus；cfg 负责控制“多高的意图分数才算命中”和
// “每个意图要多召回多少条候选”。
//
// 例子：MinIntentScore=0.8、TopKMultiplier=2、sc.TopK=5 时，
// 单个意图会先从 Milvus 召回 10 条候选，后面再交给后处理器去重和排序。
func NewIntentDirectedChannel(retriever *MilvusRetriever, cfg config.IntentDirectedChannelConfig) *IntentDirectedChannel {
	return &IntentDirectedChannel{retriever: retriever, config: cfg}
}

// Name 返回通道名称。
//
// 这个名称会写入 SearchChannelResult.ChannelName，方便日志、调试和结果归因。
// 例子：后处理阶段看到 chunk 来自 "intent_directed"，就知道它是定向通道召回的。
func (c *IntentDirectedChannel) Name() string { return "intent_directed" }

// Priority 返回通道优先级，数值越小优先级越高。
//
// 定向通道优先级是 1，高于兜底通道的 10。后处理器如果遇到重复 chunk，
// 可以优先保留更高优先级或更高置信度通道的结果。
func (c *IntentDirectedChannel) Priority() int { return 1 }

// directedTask 是一次具体的定向检索任务。
//
// 它把“查询文本”和“要查的意图”绑定在一起。
// 例子：子问题“如何安装产品 A？”命中 KB=101 的“产品 A 手册”意图，
// 就会生成一个 directedTask{subQuestion: "如何安装产品 A？", intent: 产品 A 手册}。
type directedTask struct {
	subQuestion string
	intent      IntentCandidate
}

// IsEnabled 判断是否存在足够明确的 KB 意图。
//
// 判断规则：
//   - 如果有 SubIntents，就逐个检查每个子问题的候选意图。
//   - 只要发现 Kind=KB 且 Score >= MinIntentScore，就启用本通道。
//   - 如果没有 SubIntents，就退回检查 IntentGroup.KbIntents。
//
// 例子：MinIntentScore=0.8，有两个子问题：
//   - “产品 A 怎么安装？” -> KB 意图 0.92
//   - “你是谁？” -> SYSTEM 意图 0.95
//
// 因为第一个子问题有 0.92 的 KB 意图，本函数返回 true。
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

// Search 按“子问题 + KB 意图”并行检索。
//
// 执行流程：
//   - 先调用 buildTasks，把 SearchContext 展开成多个 directedTask。
//   - 每个 task 启动一个 goroutine，并行查询对应 collection。
//   - Milvus 返回的 chunk 只有 doc_id 等信息，这里会把 task.intent.KbID 回填到 chunk.KbID。
//   - 单个 task 查询失败会被忽略，不会让整个通道失败。
//
// 例子：sc.TopK=5、TopKMultiplier=2，且有两个高分意图：
//   - 子问题“产品 A 怎么安装？” -> KB=101，collection="kb_101"，score=0.92
//   - 子问题“保修多久？” -> KB=202，collection="kb_202"，score=0.88
//
// 本函数会并行执行：
//   - retriever.Search(ctx, "kb_101", "产品 A 怎么安装？", 10)
//   - retriever.Search(ctx, "kb_202", "保修多久？", 10)
//
// 单个检索任务失败会被跳过，不影响其它任务。返回结果会补齐 KbID，
// Confidence 使用参与检索任务的意图平均分。
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
			// 如果意图没有指定集合，则使用该 KB 的默认集合。
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

// buildTasks 把搜索上下文展开成定向检索任务。
//
// 优先使用 SubIntents，因为它保留了“哪个子问题命中了哪个意图”的关系。
// 如果 SubIntents 为空，就用主问题 sc.Question 查询所有高分 KB 意图，兼容旧数据流。
//
// 例子：MinIntentScore=0.8，SubIntents 中有：
//   - 子问题 A -> KB1 0.9、SYSTEM 0.7
//   - 子问题 B -> KB2 0.6、KB3 0.85
//
// 本函数会生成两个任务：
//   - 用子问题 A 查 KB1
//   - 用子问题 B 查 KB3
//
// KB2 分数 0.6 低于阈值，SYSTEM 不是 KB 类型，所以都会被跳过。
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

// VectorGlobalChannel 是兜底向量检索通道。
//
// 它适合“还不知道具体该查哪个知识库意图，但仍然要尽量找资料”的场景。
// 比如用户指定了 KB1、KB2，但某个子问题没有命中高分 KB 意图，本通道会用该子问题
// 去 KB1、KB2 的默认 collection 都查一遍。
type VectorGlobalChannel struct {
	retriever *MilvusRetriever
	kbRepo    knowledge.KBRepo
	config    config.VectorGlobalChannelConfig
}

// NewVectorGlobalChannel 创建兜底向量检索通道。
//
// retriever 负责访问 Milvus；kbRepo 负责把用户传入的知识库 ID 查出来；
// cfg 负责控制“子问题是否已被定向通道覆盖”和“每个集合召回多少候选”。
//
// 例子：用户传入 KbIDs=[101, 202]，本通道会先通过 kbRepo 校验这两个知识库，
// 再把它们转换成默认 collection 名称。
func NewVectorGlobalChannel(retriever *MilvusRetriever, kbRepo knowledge.KBRepo, cfg config.VectorGlobalChannelConfig) *VectorGlobalChannel {
	return &VectorGlobalChannel{retriever: retriever, kbRepo: kbRepo, config: cfg}
}

// Name 返回通道名称。
//
// 这个名称会写入 SearchChannelResult.ChannelName。
// 例子：结果来自 "vector_global" 时，说明它是兜底通道召回的。
func (c *VectorGlobalChannel) Name() string { return "vector_global" }

// Priority 返回通道优先级，数值越小优先级越高。
//
// 兜底通道优先级是 10，低于定向通道。它主要补充召回，不应该盖过明确意图召回。
func (c *VectorGlobalChannel) Priority() int { return 10 }

// globalTask 是一次具体的兜底检索任务。
//
// 它表示“用 query 去 collection 查，并把结果归到 kbID”。
// 例子：用户指定 KB=101，未覆盖子问题是“是否支持退款？”，
// 就会生成 globalTask{query: "是否支持退款？", collection: "kb_101", kbID: 101}。
type globalTask struct {
	query      string
	collection string
	kbID       int64
}

// IsEnabled 判断是否需要兜底检索。
//
// 判断规则：
//   - 如果 IntentGroup.AllSystemOnly=true，说明是纯系统问题，不需要查知识库，返回 false。
//   - 如果没有 SubIntents，说明没有子问题级路由信息，返回 true，用主问题整体兜底。
//   - 如果有 SubIntents，只要存在一个“未被高置信度 KB 意图覆盖”的子问题，返回 true。
//
// 例子：ConfidenceThreshold=0.8，有 3 个子问题：
//   - A -> KB 意图 0.91，已覆盖
//   - B -> KB 意图 0.82，已覆盖
//   - C -> KB 意图 0.45，未覆盖
//
// 本函数返回 true。后续 Search 只会为 C 做兜底检索，不会重复检索 A 和 B。
func (c *VectorGlobalChannel) IsEnabled(sc SearchContext) bool {
	if sc.IntentGroup.AllSystemOnly {
		return false
	}
	// 没有子问题级路由信息，整体兜底
	if len(sc.SubIntents) == 0 {
		return true
	}
	for _, sq := range sc.SubIntents {
		if !c.isSubQuestionCovered(sq) {
			return true
		}
	}
	return false // 所有子问题都被高置信度意图覆盖
}

// isSubQuestionCovered 判断子问题是否已有高置信度 KB 意图负责检索。
//
// 只看 KB 类型意图，SYSTEM 和 MCP 都不算覆盖。
//
// 例子：ConfidenceThreshold=0.8：
//   - Candidates=[{Kind: KB, Score: 0.85}] -> true
//   - Candidates=[{Kind: KB, Score: 0.60}] -> false
//   - Candidates=[{Kind: SYSTEM, Score: 0.95}] -> false
func (c *VectorGlobalChannel) isSubQuestionCovered(sq SubQuestionIntent) bool {
	for _, intent := range sq.Candidates {
		if intent.Kind == IntentKindKB && intent.Score >= c.config.ConfidenceThreshold {
			return true
		}
	}
	return false
}

// Search 为未覆盖的子问题执行兜底检索。
//
// 执行流程：
//   - 先把 sc.KbIDs 转成默认 collection 列表。
//   - 再找出需要兜底的查询文本。
//   - 将“查询文本 × collection”展开成多个 globalTask。
//   - 每个 task 并行查询 Milvus，并把结果回填到对应 KbID。
//
// 例子：sc.KbIDs=[101, 202]、sc.TopK=5、TopKMultiplier=3，
// 子问题 A 已被高分 KB 意图覆盖，子问题 B“是否支持退款？”未覆盖。
//
// 本函数会并行执行：
//   - retriever.Search(ctx, "kb_101", "是否支持退款？", 15)
//   - retriever.Search(ctx, "kb_202", "是否支持退款？", 15)
//
// SubIntents 为空时，会用主问题查询所有指定 KB。单个任务失败会被跳过。
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

// buildTasks 把“未覆盖查询 × 指定 KB 默认集合”展开成检索任务。
//
// 这是一个笛卡尔积展开：每个需要兜底的查询，都会去每个指定 KB 的默认集合查。
//
// 例子：
//   - 未覆盖查询：["是否支持退款？", "发票怎么开？"]
//   - collections：{"kb_101": 101, "kb_202": 202}
//
// 本函数会生成 4 个任务：
//   - "是否支持退款？" × "kb_101"
//   - "是否支持退款？" × "kb_202"
//   - "发票怎么开？" × "kb_101"
//   - "发票怎么开？" × "kb_202"
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

// uncoveredQueries 返回需要兜底检索的查询文本。
//
// 返回规则：
//   - 没有 SubIntents：返回 sc.Question，表示用主问题整体兜底。
//   - 有 SubIntents：只返回没有被高置信度 KB 意图覆盖的子问题。
//
// 例子：ConfidenceThreshold=0.8：
//   - 子问题 A -> KB 0.9，已覆盖，不返回。
//   - 子问题 B -> KB 0.4，未覆盖，返回 B。
//   - 子问题 C -> SYSTEM 0.95，不算 KB 覆盖，返回 C。
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

// loadCollections 根据知识库 ID 加载默认 Milvus collection。
//
// 返回两个 map：
//   - cols：给 buildTasks 用，表示需要检索哪些 collection。
//   - kbMap：给 Search 回填 KbID 用，表示某个 collection 属于哪个 KB。
//
// 例子：kbIDs=[101, 202]，并且两个知识库都存在，则返回：
//   - cols={"kb_101": 101, "kb_202": 202}
//   - kbMap={"kb_101": 101, "kb_202": 202}
//
// 如果某个 KB ID 查不到，当前实现会跳过它，继续处理其它 KB。
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
