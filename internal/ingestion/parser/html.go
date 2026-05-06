package parser

import (
	"context"
	"strings"
)

// HtmlDocumentParser 简单剥 < > 标签。生产应改用 golang.org/x/net/html 的 tokenizer。
type HtmlDocumentParser struct{}

func NewHtmlDocumentParser() *HtmlDocumentParser { return &HtmlDocumentParser{} }

func (HtmlDocumentParser) Type() ParserType { return ParserTypeHTML }

func (HtmlDocumentParser) Supports(mimeType string) bool {
	return strings.Contains(strings.ToLower(mimeType), "html")
}

func (HtmlDocumentParser) Parse(_ context.Context, data []byte, _ string, _ string) (*ParseResult, error) {
	return &ParseResult{Text: stripHTML(string(data)), Metadata: map[string]string{}}, nil
}

func stripHTML(html string) string {
	var b strings.Builder
	inTag := false

	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}

	return CleanupText(b.String())
}
