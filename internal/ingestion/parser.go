package ingestion

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/internal/ingestion/parser"
)

type ParserNode struct {
	selector *parser.DocumentParserSelector
}

func NewParserNode(selector *parser.DocumentParserSelector) *ParserNode {
	return &ParserNode{selector: selector}
}

func (n *ParserNode) Name() string { return "parser" }

func (n *ParserNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if len(ic.RawBytes) == 0 {
		return Fail(fmt.Errorf("parser: no raw bytes to parse"))
	}

	if ic.MimeType == "" {
		ic.MimeType = parser.DetectMimeType(ic.RawBytes, ic.Source.FileName)
	}

	res, err := n.selector.Parse(ctx, ic.RawBytes, ic.MimeType, ic.Source.FileName)
	if err != nil {
		return Fail(fmt.Errorf("parser: %w", err))
	}

	ic.RawText = res.Text
	return OK(fmt.Sprintf("parsed %d chars (%s) using mime=%s", len([]rune(res.Text)), res.Metadata["tika.contentType"], ic.MimeType))
}
