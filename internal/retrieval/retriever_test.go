package retrieval

import (
	"testing"

	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/stretchr/testify/assert"
)

func TestParseSearchResults_Empty(t *testing.T) {
	chunks := parseSearchResults(nil, "kb_1")
	assert.Empty(t, chunks)
}

func TestParseSearchResults_NoResults(t *testing.T) {
	chunks := parseSearchResults([]milvusclient.SearchResult{}, "kb_1")
	assert.Empty(t, chunks)
}
