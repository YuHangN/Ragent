package parser

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/YuHangN/ragent-go/pkg/apperror"
)

// TextDocumentParser 处理 text/plain, application/json 这类纯 UTF-8 文本。
type TextDocumentParser struct{}

func NewTextDocumentParser() *TextDocumentParser { return &TextDocumentParser{} }

func (TextDocumentParser) Type() ParserType { return ParserTypeText }

func (TextDocumentParser) Supports(mimeType string) bool {
	lower := strings.ToLower(mimeType)
	return strings.HasPrefix(lower, "text/plain") || lower == "application/json"
}

func (TextDocumentParser) Parse(_ context.Context, data []byte, _ string, _ string) (*ParseResult, error) {
	if !utf8.Valid(data) {
		return nil, apperror.NewClientMsg("文本不是合法 UTF-8")
	}

	return &ParseResult{Text: CleanupText(string(data)), Metadata: map[string]string{}}, nil
}
