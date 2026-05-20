package aiclient

import (
	"context"
)

// RerankClient 定义检索片段重排 provider 适配器需要实现的接口。
type RerankClient interface {
	Provider() Provider
	Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int, target *ModelTarget) ([]RetrievedChunk, error)
}

// NoopRerankClient 是不调用外部服务的重排器，按原顺序返回候选片段。
type NoopRerankClient struct{}

// NewNoopRerankClient 构造一个用于无外部重排模型场景的空操作重排器。
func NewNoopRerankClient() *NoopRerankClient { return &NoopRerankClient{} }

// Provider 返回 ProviderNoop，供路由表匹配。
func (*NoopRerankClient) Provider() Provider { return ProviderNoop }

// Rerank 保持输入顺序，并在 topN 有效时截断结果。
func (*NoopRerankClient) Rerank(_ context.Context, _ string, candidates []RetrievedChunk, topN int, _ *ModelTarget) ([]RetrievedChunk, error) {
	if topN > 0 && topN < len(candidates) {
		return candidates[:topN], nil
	}
	return candidates, nil
}
