package rag

import (
	"context"
	"sort"
	"sync"
)

// MultiChannelEngine 负责调度多个检索通道。
//
// 它本身不直接访问 Milvus，也不决定某个 chunk 是否最终保留。它只做编排：
//   - 先问每个 SearchChannel 是否应该启用。
//   - 并行执行启用的通道。
//   - 把所有通道召回的 chunk 交给 PostProcessor 链处理。
//
// 例子：定向通道召回了产品手册片段，兜底通道召回了售后政策片段，
// 引擎会把两边结果合并后交给去重、rerank 等后处理器。
type MultiChannelEngine struct {
	channels   []SearchChannel
	processors []PostProcessor
}

// NewMultiChannelEngine 创建多通道检索引擎。
//
// channels 会按 Priority 从小到大排序；processors 会按 Order 从小到大排序。
// 排序发生在创建阶段，后续 Retrieve 就可以直接按固定顺序使用。
//
// 例子：
//   - channels=[vector_global(Priority=10), intent_directed(Priority=1)]
//   - processors=[rerank(Order=20), dedup(Order=10)]
//
// 创建后内部顺序会变成：
//   - channels=[intent_directed, vector_global]
//   - processors=[dedup, rerank]
func NewMultiChannelEngine(channels []SearchChannel, processors []PostProcessor) *MultiChannelEngine {
	// Priority 越小越靠前，方便结果归因和后处理阶段判断优先级。
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
//   - 调用每个通道的 IsEnabled，筛出本次应该运行的通道。
//   - 并行调用每个启用通道的 Search。
//   - 单个通道失败会被跳过，不影响其它通道结果。
//   - 将所有 chunk 依次交给后处理器链，比如去重、排序、截断。
//
// 例子：引擎中有两个通道：
//   - intent_directed：IsEnabled=true，返回 chunk A、B。
//   - vector_global：IsEnabled=true，返回 chunk B、C。
//
// 如果后处理器包含 DeduplicationProcessor，最终可能返回 A、B、C，
// 其中重复的 B 只保留一次。具体保留哪一个由后处理器根据通道置信度等规则决定。
func (e *MultiChannelEngine) Retrieve(ctx context.Context, sc SearchContext) ([]RetrievedChunk, error) {
	if sc.TopK <= 0 {
		sc.TopK = 5
	}

	// 只运行适合当前请求的通道。
	var activeChannels []SearchChannel
	for _, ch := range e.channels {
		if ch.IsEnabled(sc) {
			activeChannels = append(activeChannels, ch)
		}
	}

	// 各通道互不依赖，可以并行召回。
	type chanResult struct {
		result SearchChannelResult
		err    error
	}
	resCh := make(chan chanResult, len(activeChannels))
	var wg sync.WaitGroup

	for _, ch := range activeChannels {
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

	// 收集所有成功通道的结果；失败通道被忽略。
	var allResults []SearchChannelResult
	var allChunks []RetrievedChunk
	for res := range resCh {
		if res.err != nil {
			continue // 单通道失败不影响其他通道
		}
		allResults = append(allResults, res.result)
		allChunks = append(allChunks, res.result.Chunks...)
	}

	// 后处理器按 Order 顺序串行执行，每一步都接收上一步的输出。
	chunks := allChunks
	for _, proc := range e.processors {
		chunks = proc.Process(chunks, allResults, sc)
	}

	return chunks, nil
}
