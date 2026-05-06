package parser

import (
	"context"
	"strings"
)

// MarkdownDocumentParser 保留 markdown 原文（embedding 模型对结构化 markdown 友好）。
type MarkdownDocumentParser struct{}

func NewMarkdownDocumentParser() *MarkdownDocumentParser { return &MarkdownDocumentParser{} }

func (MarkdownDocumentParser) Type() ParserType { return ParserTypeMarkdown }

func (MarkdownDocumentParser) Supports(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(mimeType), "text/markdown")
}

func (MarkdownDocumentParser) Parse(_ context.Context, data []byte, _ string, _ string) (*ParseResult, error) {
	return &ParseResult{Text: CleanupText(string(data)), Metadata: map[string]string{}}, nil
}
