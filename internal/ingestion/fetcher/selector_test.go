package fetcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubFetcher struct {
	t SourceType
}

func (s stubFetcher) Type() SourceType { return s.t }
func (s stubFetcher) Fetch(_ context.Context, req FetchRequest) (*FetchResult, error) {
	return &FetchResult{Bytes: []byte("stub:" + req.Location)}, nil
}

func TestFetcherSelector_Select(t *testing.T) {
	sel := NewFetcherSelector([]DocumentFetcher{
		stubFetcher{t: SourceLocal},
		stubFetcher{t: SourceHTTP},
	})

	f, err := sel.Select(SourceLocal)
	require.NoError(t, err)
	assert.Equal(t, SourceLocal, f.Type())

	_, err = sel.Select(SourceFeishu)
	require.Error(t, err)
}
