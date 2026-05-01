package aiclient

import (
	"context"
)

type RerankClient interface {
	Provider() Provider
	Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int, target *ModelTarget) ([]RetrievedChunk, error)
}

type NoopRerankClient struct{}

func NewNoopRerankClient() *NoopRerankClient { return &NoopRerankClient{} }

func (*NoopRerankClient) Provider() Provider { return ProviderNoop }

func (*NoopRerankClient) Rerank(_ context.Context, _ string, candidates []RetrievedChunk, topN int, _ *ModelTarget) ([]RetrievedChunk, error) {
	if topN > 0 && topN < len(candidates) {
		return candidates[:topN], nil
	}
	return candidates, nil
}
