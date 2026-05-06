package parser

import (
	"context"
)

type ParserType string

const (
	ParserTypeText     ParserType = "text"
	ParserTypeMarkdown ParserType = "markdown"
	ParserTypeHTML     ParserType = "html"
	ParserTypeTika     ParserType = "tika"
)

type ParseResult struct {
	Text     string            // 主文本内容
	Metadata map[string]string // 解析期间提取的元数据（如 PDF 标题、页数）
}

type DocumentParser interface {
	Type() ParserType
	Supports(mimeType string) bool
	Parse(ctx context.Context, data []byte, mimeType string, fileName string) (*ParseResult, error)
}
