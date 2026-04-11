package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeName_TrimsSpaces(t *testing.T) {
	got := NormalizeName("  我的知识库  ")
	assert.Equal(t, "我的知识库", got)
}

func TestNormalizeName_CollapseInternalSpaces(t *testing.T) {
	got := NormalizeName("知识  库  test")
	assert.Equal(t, "知识库test", got)
}

func TestBuildCollectionName(t *testing.T) {
	name := BuildCollectionName(42)
	assert.Equal(t, "kb_42", name)
}
