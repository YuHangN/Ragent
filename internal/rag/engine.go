package rag

import (
	"context"
	"sort"
	"sync"
)

// MultiChannelEngine 并行执行多个检索通道，再通过后处理链过滤排序。
type MultiChannelEngine struct {
	channels   []SearchChannel
	processors []PostProcessor
}

func NewMultiChannelEngine(channels []SearchChannel, processors []PostProcessor) *MultiChannelEngine {
	// 按 Priority 升序排列通道
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Priority() < channels[j].Priority()
	})

	// 按 Order 升序排列后处理器
	sort.Slice(processors, func(i, j int) bool {
		return processors[i].Order() < processors[j].Order()
	})

	return &MultiChannelEngine{channels: channels, processors: processors}
}

// Retrieve 执行完整的多通道检索 + 后处理链，返回最终 chunk 列表。
func (e *MultiChannelEngine) Retrieve(ctx context.Context, sc SearchContext) ([]RetrievedChunk, error) {
	if sc.TopK <= 0 {
		sc.TopK = 5
	}

	// 1. 过滤出启用的通道
	var activeChannels []SearchChannel
	for _, ch := range e.channels {
		if ch.IsEnabled(sc) {
			activeChannels = append(activeChannels, ch)
		}
	}

	// 2. 并行执行各通道
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

	// 3. 收集结果
	var allResults []SearchChannelResult
	var allChunks []RetrievedChunk
	for res := range resCh {
		if res.err != nil {
			continue // 单通道失败不影响其他通道
		}
		allResults = append(allResults, res.result)
		allChunks = append(allChunks, res.result.Chunks...)
	}

	// 4. 依次执行后处理器链
	chunks := allChunks
	for _, proc := range e.processors {
		chunks = proc.Process(chunks, allResults, sc)
	}

	return chunks, nil
}
