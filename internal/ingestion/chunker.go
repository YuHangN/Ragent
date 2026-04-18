package ingestion

import (
	"context"
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/idgen"
)

// ChunkerNode splits ic.RawText (or ic.EnhancedText if set) into VectorChunks.
type ChunkerNode struct {
	strategy ChunkerStrategy
	size     int // chunk size in runes
	overlap  int // overlap size in runes
}

// ChunkerStrategy splits text into content slices.
type ChunkerStrategy interface {
	Name() string
	Chunk(text string, size, overlap int) []string
}

func NewChunkerNode(strategy ChunkerStrategy, size, overlap int) *ChunkerNode {
	if size <= 0 {
		size = 512
	}
	if overlap < 0 {
		overlap = 0
	}

	return &ChunkerNode{strategy: strategy, size: size, overlap: overlap}
}

func (n *ChunkerNode) Name() string { return "chunker" }

func (n *ChunkerNode) Execute(_ context.Context, ic *IngestionContext) NodeResult {
	text := ic.RawText
	if ic.EnhancedText != "" {
		text = ic.EnhancedText // use AI-enhanced text if available (Phase 6)
	}
	if strings.TrimSpace(text) == "" {
		return Fail(fmt.Errorf("chunker: no text to chunk"))
	}

	contents := n.strategy.Chunk(text, n.size, n.overlap)
	chunks := make([]VectorChunk, 0, len(contents))
	for i, c := range contents {
		if strings.TrimSpace(c) == "" {
			continue
		}
		chunks = append(chunks, VectorChunk{
			ChunkID:  idgen.NewStringID(),
			Index:    i,
			Content:  c,
			Metadata: make(map[string]any),
		})
	}

	ic.Chunks = chunks
	return OK(fmt.Sprintf("chunked into %d segments using %s strategy", len(chunks), n.strategy.Name()))
}

// FixedSizeChunker cuts text into rune-counted windows with overlap.
type FixedSizeChunker struct{}

func (FixedSizeChunker) Name() string { return "fixed_size" }

func (FixedSizeChunker) Chunk(text string, size, overlap int) []string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return nil
	}
	if size >= n {
		return []string{string(runes)}
	}
	if overlap >= size {
		overlap = size - 1
	}

	var chunks []string
	start := 0
	for start < n {
		end := start + size
		if end > n {
			end = n
		}

		// Try to snap end to a sentence boundary within the overlap window.
		end = snapToBoundary(runes, start, end, overlap)

		chunks = append(chunks, string(runes[start:end]))

		if end >= n {
			break
		}
		// Next window starts (end - overlap) runes back.
		next := end - overlap
		if next <= start {
			next = end // safety: always advance
		}
		start = next
	}
	return chunks
}

// snapToBoundary looks back up to `overlap` runes from targetEnd to find
func snapToBoundary(runes []rune, start, targetEnd, overlap int) int {
	maxLook := overlap
	if maxLook > targetEnd-start {
		maxLook = targetEnd - start
	}
	// 1. newline
	for i := 0; i <= maxLook; i++ {
		pos := targetEnd - i - 1
		if pos <= start {
			break
		}
		if runes[pos] == '\n' {
			return pos + 1
		}
	}
	// 2. CJK sentence-end punctuation
	for i := 0; i <= maxLook; i++ {
		pos := targetEnd - i - 1
		if pos <= start {
			break
		}
		switch runes[pos] {
		case '。', '！', '？':
			return pos + 1
		}
	}
	// 3. English sentence-end (only if next char is space or end-of-slice)
	for i := 0; i <= maxLook; i++ {
		pos := targetEnd - i - 1
		if pos <= start {
			break
		}
		switch runes[pos] {
		case '.', '!', '?':
			next := pos + 1
			if next >= len(runes) || runes[next] == ' ' || runes[next] == '\n' {
				return next
			}
		}
	}
	return targetEnd
}

// ────────────────────────────────────────────────────────────────────
// ParagraphChunker
// ────────────────────────────────────────────────────────────────────

// ParagraphChunker splits on blank lines (two or more newlines).
type ParagraphChunker struct{}

func (ParagraphChunker) Name() string { return "paragraph" }

func (ParagraphChunker) Chunk(text string, _, _ int) []string {
	// Split on two or more consecutive newlines.
	raw := strings.Split(text, "\n\n")
	var out []string
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
