package retrieval

import (
	"context"
	"sort"
	"sync"

	"github.com/YuHangN/ragent-go/internal/intent"
)

// MultiChannelEngine 负责调度多个检索通道。
//
// 它本身不直接访问 Milvus，也不决定某个 chunk 是否最终保留。它只做编排：
//   - 按通道 Priority 把通道分成多个优先级层（tier）。
//   - 每一层内的通道并行执行。
//   - 前一层执行完后，把“哪些子问题被处理过但查空了”写回 SearchContext。
//   - 下一层再根据更新后的 SearchContext 判断自己是否需要兜底。
//   - 最后把所有通道召回的 chunk 交给 PostProcessor 链处理。
//
// tier 可以理解为“第几轮检索”。Priority 越小，tier 越靠前。
// 例子：intent_directed 的 Priority=1，vector_global 的 Priority=10，
// 所以引擎会先跑 intent_directed；如果某些子问题没查到结果，再让 vector_global
// 判断是否需要补查。
type MultiChannelEngine struct {
	channels   []SearchChannel
	processors []PostProcessor
}

// NewMultiChannelEngine 创建多通道检索引擎。
//
// channels 会按 Priority 从小到大排序。Priority 不是“分数”，而是通道执行层级：
// 数字越小越早执行。相同 Priority 的通道属于同一个 tier，会在同一轮里并行执行。
//
// processors 会按 Order 从小到大排序。Order 表示后处理器执行顺序：
// 数字越小越早执行。
//
// 例子：
//   - channels=[vector_global(Priority=10), intent_directed(Priority=1), custom(Priority=10)]
//   - processors=[rerank(Order=10), dedup(Order=1)]
//
// 创建后内部顺序会变成：
//   - channels=[intent_directed, vector_global, custom]
//   - processors=[dedup, rerank]
//
// Retrieve 时会再把 channels 分成两个 tier：
//   - tier 1: [intent_directed]
//   - tier 2: [vector_global, custom]
func NewMultiChannelEngine(channels []SearchChannel, processors []PostProcessor) *MultiChannelEngine {
	// 先排序，后面 groupByPriority 才能把相同 Priority 的通道放进同一个 tier。
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Priority() < channels[j].Priority()
	})

	// Order 越小越先执行，比如通常先去重，再做 rerank。
	sort.Slice(processors, func(i, j int) bool {
		return processors[i].Order() < processors[j].Order()
	})

	return &MultiChannelEngine{channels: channels, processors: processors}
}

// Retrieve 执行一次完整检索，并返回最终 chunk 列表。
//
// 执行流程：
//   - TopK <= 0 时默认改成 5。
//   - 把通道按 Priority 分成多个优先级层（tier）。
//   - 按 tier 顺序执行：先跑 Priority 小的层，再跑 Priority 大的层。
//   - 每个 tier 内部的通道互不依赖，所以并行执行。
//   - 每个 tier 结束后，记录“本层处理过但没查到结果”的子问题。
//   - 下一层可以读取这些子问题，决定是否做兜底检索。
//   - 所有 tier 结束后，把全部 chunk 交给后处理器链。
//
// tier 的例子：
//   - tier 1: intent_directed(Priority=1)
//   - tier 2: vector_global(Priority=10)
//
// 假设问题被拆成两个子问题：
//   - “产品 A 怎么安装？”
//   - “保修多久？”
//
// tier 1 的 intent_directed 会先查高置信度意图对应的集合。
// 如果“产品 A 怎么安装？”查到了 chunk，而“保修多久？”没有查到，
// 引擎会把“保修多久？”写入 sc.PriorTierEmptySubQuestions。
// tier 2 的 vector_global 看到这个标记后，就可以只为“保修多久？”做全局兜底。
func (e *MultiChannelEngine) Retrieve(ctx context.Context, sc SearchContext) ([]RetrievedChunk, error) {
	if sc.TopK <= 0 {
		sc.TopK = 5
	}

	tiers := groupByPriority(e.channels)

	var allResults []SearchChannelResult
	var allChunks []RetrievedChunk

	for _, tier := range tiers {
		// 每一层都重新调用 IsEnabled。
		// 这样后面的 tier 可以看到前一层写入的 PriorTierEmptySubQuestions。
		var active []SearchChannel
		for _, ch := range tier {
			if ch.IsEnabled(sc) {
				active = append(active, ch)
			}
		}
		if len(active) == 0 {
			continue
		}

		tierResults := e.runTier(ctx, active, sc)
		allResults = append(allResults, tierResults...)
		for _, r := range tierResults {
			allChunks = append(allChunks, r.Chunks...)
		}

		// 用本 tier 的命中情况推导下一 tier 要关注的“查空子问题”。
		// 这里每一层都会覆盖旧值，而不是一直累加；下一层只关心刚刚跑完的上一层。
		sc.PriorTierEmptySubQuestions = deriveEmptySubQuestions(sc.SubIntents, tierResults)
	}

	// 后处理器按 Order 顺序串行执行，每一步都接收上一步的输出。
	chunks := allChunks
	for _, proc := range e.processors {
		chunks = proc.Process(chunks, allResults, sc)
	}

	return chunks, nil
}

// runTier 并行执行同一个优先级层里的通道。
//
// 同一个 tier 表示这些通道的 Priority 相同，它们之间没有先后依赖。
// 因此可以同时调用 Search，减少总耗时。
//
// 例子：某个 tier 中有两个通道：
//   - keyword_channel(Priority=5)
//   - vector_channel(Priority=5)
//
// runTier 会同时启动两个 goroutine。哪个先返回不重要，最终都会收集到 results 中。
// 如果其中一个通道报错，会跳过它；另一个成功通道的结果仍然保留。
func (e *MultiChannelEngine) runTier(ctx context.Context, channels []SearchChannel, sc SearchContext) []SearchChannelResult {
	type chanResult struct {
		result SearchChannelResult
		err    error
	}
	resCh := make(chan chanResult, len(channels))
	var wg sync.WaitGroup

	for _, ch := range channels {
		ch := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := ch.Search(ctx, sc)
			resCh <- chanResult{result: result, err: err}
		}()
	}
	wg.Wait()
	close(resCh)

	var results []SearchChannelResult
	for res := range resCh {
		if res.err != nil {
			continue // 单通道失败不影响其他通道
		}
		results = append(results, res.result)
	}
	return results
}

// groupByPriority 把已按 Priority 排好序的通道分成多个优先级层。
//
// tier 的意思是“一轮检索”。同一个 tier 内的通道 Priority 相同，会并行执行；
// 不同 tier 之间按 Priority 从小到大串行执行。
//
// 例子：输入通道顺序是：
//   - intent_directed(Priority=1)
//   - keyword(Priority=5)
//   - vector_global(Priority=10)
//   - custom_global(Priority=10)
//
// 返回结果是：
//   - tier 1: [intent_directed]
//   - tier 2: [keyword]
//   - tier 3: [vector_global, custom_global]
//
// 注意：这个函数假设 channels 已经按 Priority 升序排列。
// NewMultiChannelEngine 已经做了排序，所以正常调用路径满足这个前提。
func groupByPriority(channels []SearchChannel) [][]SearchChannel {
	if len(channels) == 0 {
		return nil
	}
	var tiers [][]SearchChannel
	curPri := channels[0].Priority()
	cur := []SearchChannel{channels[0]}
	for _, ch := range channels[1:] {
		if ch.Priority() == curPri {
			cur = append(cur, ch)
			continue
		}
		tiers = append(tiers, cur)
		curPri = ch.Priority()
		cur = []SearchChannel{ch}
	}
	tiers = append(tiers, cur)
	return tiers
}

// deriveEmptySubQuestions 推导“本层查空的子问题”。
//
// 通道可以在 SearchChannelResult.PerSubQuestionHits 里报告每个子问题命中了多少 chunk。
// 本函数会查看当前 tier 的所有结果：
//   - 如果某个子问题在 PerSubQuestionHits 中出现过，说明本层通道处理过它。
//   - 如果出现过，但命中数加起来是 0，说明本层查了但没查到。
//   - 这种子问题会被写入返回 map，供下一层做兜底。
//
// 例子：有两个子问题：
//   - A = “产品 A 怎么安装？”
//   - B = “保修多久？”
//
// 本层通道返回：
//   - PerSubQuestionHits[A] = 2
//   - PerSubQuestionHits[B] = 0
//
// deriveEmptySubQuestions 会返回 {"保修多久？": true}。
// 如果某个通道没有填写 PerSubQuestionHits，本函数不会根据它猜测查空情况。
func deriveEmptySubQuestions(subs []intent.SubQuestionIntent, results []SearchChannelResult) map[string]bool {
	if len(subs) == 0 || len(results) == 0 {
		return nil
	}
	empty := make(map[string]bool)
	for _, sq := range subs {
		hits := 0
		seen := false
		for _, r := range results {
			if r.PerSubQuestionHits == nil {
				continue
			}
			if v, ok := r.PerSubQuestionHits[sq.SubQuestion]; ok {
				seen = true
				hits += v
			}
		}
		if seen && hits == 0 {
			empty[sq.SubQuestion] = true
		}
	}
	if len(empty) == 0 {
		return nil
	}
	return empty
}
