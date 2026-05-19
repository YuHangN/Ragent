// Package retrieval 包含检索增强生成（RAG）的核心逻辑。
//
// 本文件定义"检索通道"。通道只负责召回候选文档片段，不负责生成答案：
//   - IntentDirectedChannel：意图明确时，只查相关知识库。
//   - VectorGlobalChannel：意图不明确时，在用户指定的知识库中兜底查。
package retrieval

import (
	"context"
	"fmt"
	"sync"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/internal/intent"
	"github.com/YuHangN/ragent-go/internal/knowledge"
)

// IntentDirectedChannel 根据意图结果做定向检索。
//
// 它适合"已经知道应该查哪个知识库"的场景。比如子问题"产品 A 怎么安装？"
// 命中了"产品 A 手册"意图，分数达到阈值，本通道就只查产品 A 对应的 collection，
// 不会把请求发到所有知识库。
type IntentDirectedChannel struct {
	retriever *MilvusRetriever
	config    config.IntentDirectedChannelConfig
}

func NewIntentDirectedChannel(retriever *MilvusRetriever, cfg config.IntentDirectedChannelConfig) *IntentDirectedChannel {
	return &IntentDirectedChannel{retriever: retriever, config: cfg}
}

func (c *IntentDirectedChannel) Name() string { return "intent_directed" }

// Priority 数值越小优先级越高；定向通道 1 高于兜底通道 10。
func (c *IntentDirectedChannel) Priority() int { return 1 }

type directedTask struct {
	subQuestion string
	cand        intent.Candidate
}

// IsEnabled 判断是否存在足够明确的 KB 意图。
//
// 优先用 SubIntents（子问题级路由信息）；SubIntents 为空时退回看 IntentGroup.KbIntents。
func (c *IntentDirectedChannel) IsEnabled(sc SearchContext) bool {
	if len(sc.SubIntents) > 0 {
		for _, sq := range sc.SubIntents {
			for _, cand := range sq.Candidates {
				if cand.Kind == intent.KindKB && cand.Score >= c.config.MinIntentScore {
					return true
				}
			}
		}
		return false
	}
	for _, cand := range sc.IntentGroup.KbIntents {
		if cand.Score >= c.config.MinIntentScore {
			return true
		}
	}
	return false
}

// Search 按"子问题 + KB 意图"并行检索。单 task 失败不阻断整体。
func (c *IntentDirectedChannel) Search(ctx context.Context, sc SearchContext) (SearchChannelResult, error) {
	tasks := c.buildTasks(sc)

	topKPerIntent := sc.TopK * c.config.TopKMultiplier
	if topKPerIntent <= 0 {
		topKPerIntent = sc.TopK * 2
	}

	type searchRes struct {
		subQuestion string
		chunks      []RetrievedChunk
		err         error
	}
	resCh := make(chan searchRes, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Phase 6.7：collection 永远是该 KB 的物理 collection；意图缩范围靠 partition。
			// 空 PartitionName → partitions=nil → 扫该 collection 下所有 partition（兜底）。
			col := knowledge.BuildCollectionName(task.cand.KbID)
			var partitions []string
			if task.cand.PartitionName != "" {
				partitions = []string{task.cand.PartitionName}
			}
			chunks, err := c.retriever.Search(ctx, col, partitions, task.subQuestion, topKPerIntent)
			if err != nil {
				resCh <- searchRes{subQuestion: task.subQuestion, err: err}
				return
			}
			for i := range chunks {
				chunks[i].KbID = task.cand.KbID
			}
			resCh <- searchRes{subQuestion: task.subQuestion, chunks: chunks}
		}()
	}

	wg.Wait()
	close(resCh)

	var all []RetrievedChunk
	// hits 记录每个子问题在本通道实际命中的 chunk 数。engine 据此推导 fall-through 兜底。
	hits := make(map[string]int)
	// touched 记录"本通道为该子问题至少跑了一次检索"——即使 0 命中也要落 0 进 hits，
	// 让 engine 能区分"通道没碰过"和"通道碰过但查空"。
	touched := make(map[string]bool)
	for res := range resCh {
		touched[res.subQuestion] = true
		if res.err != nil {
			continue
		}
		all = append(all, res.chunks...)
		hits[res.subQuestion] += len(res.chunks)
	}
	for sq := range touched {
		if _, ok := hits[sq]; !ok {
			hits[sq] = 0
		}
	}

	var avgScore float64
	for _, t := range tasks {
		avgScore += t.cand.Score
	}
	if len(tasks) > 0 {
		avgScore /= float64(len(tasks))
	}

	return SearchChannelResult{
		ChannelName:        c.Name(),
		Priority:           c.Priority(),
		Chunks:             all,
		Confidence:         avgScore,
		PerSubQuestionHits: hits,
	}, nil
}

// buildTasks 把搜索上下文展开成定向检索任务。
// 优先用 SubIntents 的子问题级绑定；为空时退回用主问题查所有高分 KB 意图。
func (c *IntentDirectedChannel) buildTasks(sc SearchContext) []directedTask {
	if len(sc.SubIntents) == 0 {
		var tasks []directedTask
		for _, cand := range sc.IntentGroup.KbIntents {
			if cand.Score >= c.config.MinIntentScore {
				tasks = append(tasks, directedTask{subQuestion: sc.Question, cand: cand})
			}
		}
		return tasks
	}

	var tasks []directedTask
	for _, sq := range sc.SubIntents {
		for _, cand := range sq.Candidates {
			if cand.Kind != intent.KindKB {
				continue
			}
			if cand.Score < c.config.MinIntentScore {
				continue
			}
			tasks = append(tasks, directedTask{subQuestion: sq.SubQuestion, cand: cand})
		}
	}
	return tasks
}

// ──── VectorGlobalChannel ─────────────────────────────────

// VectorGlobalChannel 是兜底向量检索通道。
//
// 适合"不知道具体该查哪个知识库意图，但仍要尽量找资料"的场景。
// 用户指定 KB1、KB2，某个子问题没命中高分 KB 意图时，用该子问题去 KB1、KB2 默认查一遍。
type VectorGlobalChannel struct {
	retriever *MilvusRetriever
	kbRepo    knowledge.KBRepo
	config    config.VectorGlobalChannelConfig
}

func NewVectorGlobalChannel(retriever *MilvusRetriever, kbRepo knowledge.KBRepo, cfg config.VectorGlobalChannelConfig) *VectorGlobalChannel {
	return &VectorGlobalChannel{retriever: retriever, kbRepo: kbRepo, config: cfg}
}

func (c *VectorGlobalChannel) Name() string { return "vector_global" }

// Priority 兜底通道 10，低于定向通道 1。
func (c *VectorGlobalChannel) Priority() int { return 10 }

type globalTask struct {
	query      string
	collection string
	kbID       int64
}

// IsEnabled 判断是否需要兜底检索。
//
// 规则：
//   - IntentGroup.AllSystemOnly=true → false（纯系统应答，不查 KB）
//   - 没有 SubIntents → true（用主问题整体兜底）
//   - 有 SubIntents → 任意一个未被高置信度 KB 覆盖的子问题，或前一 tier 实际查空的子问题，返回 true
func (c *VectorGlobalChannel) IsEnabled(sc SearchContext) bool {
	if sc.IntentGroup.AllSystemOnly {
		return false
	}
	if len(sc.SubIntents) == 0 {
		return true
	}
	for _, sq := range sc.SubIntents {
		if !c.isSubQuestionCovered(sq) {
			return true
		}
		// Phase 6.7.1：前一 tier 应该查到却空了（典型场景：intent 高分但 partition
		// 没数据，因为文档没填 targetPartition 进了 _default）→ 由本通道兜底。
		if sc.PriorTierEmptySubQuestions[sq.SubQuestion] {
			return true
		}
	}
	return false
}

func (c *VectorGlobalChannel) isSubQuestionCovered(sq intent.SubQuestionIntent) bool {
	for _, cand := range sq.Candidates {
		if cand.Kind == intent.KindKB && cand.Score >= c.config.ConfidenceThreshold {
			return true
		}
	}
	return false
}

// Search 为未覆盖的子问题做兜底检索：未覆盖查询 × 指定 KB 默认 collection 笛卡尔展开并行查。
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
			// VectorGlobal 兜底：partitions=nil 扫该 collection 下所有 partition。
			chunks, err := c.retriever.Search(ctx, task.collection, nil, task.query, topKPerCol)
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
		Confidence:  0.7,
	}, nil
}

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
			continue
		}
		// Phase 6.7.1：被高置信度覆盖但前一 tier 实际查空 → 也要兜底
		if sc.PriorTierEmptySubQuestions[sq.SubQuestion] {
			queries = append(queries, sq.SubQuestion)
		}
	}
	return queries
}

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
