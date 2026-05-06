package fetcher

import (
	"context"
	"fmt"
)

type FetcherSelector struct {
	byType map[SourceType]DocumentFetcher
}

func NewFetcherSelector(fetchers []DocumentFetcher) *FetcherSelector {
	byType := make(map[SourceType]DocumentFetcher, len(fetchers))
	for _, f := range fetchers {
		byType[f.Type()] = f
	}
	return &FetcherSelector{byType: byType}
}

func (s *FetcherSelector) Select(t SourceType) (DocumentFetcher, error) {
	f, ok := s.byType[t]
	if !ok {
		return nil, fmt.Errorf("no fetcher registered for source type %q", t)
	}
	return f, nil
}

func (s *FetcherSelector) Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error) {
	f, err := s.Select(req.Type)
	if err != nil {
		return nil, err
	}
	return f.Fetch(ctx, req)
}
