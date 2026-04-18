package ingestion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixedSizeChunker_ShortText(t *testing.T) {
	c := FixedSizeChunker{}
	chunks := c.Chunk("hello", 512, 128)
	assert.Equal(t, []string{"hello"}, chunks)
}

func TestFixedSizeChunker_ExactSplit(t *testing.T) {
	c := FixedSizeChunker{}
	chunks := c.Chunk("0123456789", 5, 0)

	assert.Len(t, chunks, 2)
	assert.Equal(t, "01234", chunks[0])
	assert.Equal(t, "56789", chunks[1])
}

func TestFixedSizeChunker_Overlap(t *testing.T) {
	c := FixedSizeChunker{}
	chunks := c.Chunk("0123456789", 6, 2)
	assert.Len(t, chunks, 2)
	assert.Equal(t, "012345", chunks[0])
	assert.Equal(t, "456789", chunks[1])
}

func TestFixedSizeChunker_BoundaryNewline(t *testing.T) {
	// size=6, overlap=3: targetEnd=6 向左看到 pos=4('\n')，第一块在 \n 处切断
	// 之后 overlap 窗口继续找 \n，最终末尾块包含 "bbbb"
	text := "aaaa\nbbbb"
	c := FixedSizeChunker{}
	chunks := c.Chunk(text, 6, 3)

	assert.Equal(t, "aaaa\n", chunks[0])
	assert.True(t, len(chunks) > 0)
	// 确认文本内容全部覆盖
	last := chunks[len(chunks)-1]
	assert.Contains(t, last, "bbbb")
}

func TestFixedSizeChunker_CJKBoundary(t *testing.T) {
	text := "你好世界。再见世界。"
	c := FixedSizeChunker{}
	chunks := c.Chunk(text, 6, 2)

	assert.True(t, len(chunks) > 0)
	for _, ch := range chunks {
		assert.True(t, len([]rune(ch)) > 0)
	}
}

func TestFixedSizeChunker_EmptyText(t *testing.T) {
	c := FixedSizeChunker{}
	assert.Nil(t, c.Chunk("", 512, 128))
}

func TestParagraphChunker_Basic(t *testing.T) {
	text := "First paragraph.\n\nSecond paragraph.\n\nThird."
	c := ParagraphChunker{}
	chunks := c.Chunk(text, 0, 0)
	assert.Equal(t, []string{
		"First paragraph.",
		"Second paragraph.",
		"Third.",
	}, chunks)
}

func TestParagraphChunker_TrimsWhitespace(t *testing.T) {
	text := "  hello  \n\n  world  "
	c := ParagraphChunker{}
	chunks := c.Chunk(text, 0, 0)
	assert.Equal(t, []string{"hello", "world"}, chunks)
}

func TestParagraphChunker_SkipsEmptyParagraphs(t *testing.T) {
	text := "a\n\n\n\nb"

	c := ParagraphChunker{}
	chunks := c.Chunk(text, 0, 0)
	assert.Equal(t, []string{"a", "b"}, chunks)
}
