package aiclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeuristicTokenCounter_Empty(t *testing.T) {
	tc := NewHeuristicTokenCounter()
	assert.Equal(t, 0, tc.Count(""))
	assert.Equal(t, 0, tc.Count("   \t\n"))
}

func TestHeuristicTokenCounter_ASCII(t *testing.T) {
	tc := NewHeuristicTokenCounter()
	assert.Equal(t, 2, tc.Count("hello"))
	assert.Equal(t, 1, tc.Count("hi"))
	assert.Equal(t, 3, tc.Count("1234567890ab"))
}

func TestHeuristicTokenCounter_CJK(t *testing.T) {
	tc := NewHeuristicTokenCounter()
	assert.Equal(t, 5, tc.Count("你好世界啊"))
	assert.Equal(t, 5, tc.Count("你好，世界"))
}

func TestHeuristicTokenCounter_Mixed(t *testing.T) {
	tc := NewHeuristicTokenCounter()
	assert.Equal(t, 4, tc.Count("hello 世界"))
}

func TestHeuristicTokenCounter_OtherScripts(t *testing.T) {
	tc := NewHeuristicTokenCounter()
	assert.Equal(t, 2, tc.Count("café"))
}

func TestHeuristicTokenCounter_MinimumIsOne(t *testing.T) {
	tc := NewHeuristicTokenCounter()
	assert.Equal(t, 1, tc.Count("a"))
}
