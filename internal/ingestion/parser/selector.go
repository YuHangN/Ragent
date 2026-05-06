package parser

import (
	"context"
	"fmt"
)

type DocumentParserSelector struct {
	parsers []DocumentParser
	byType  map[ParserType]DocumentParser
}

func NewDocumentParserSelector(parsers []DocumentParser) *DocumentParserSelector {
	byType := make(map[ParserType]DocumentParser, len(parsers))
	for _, p := range parsers {
		byType[p.Type()] = p
	}
	return &DocumentParserSelector{parsers: parsers, byType: byType}
}

func (s *DocumentParserSelector) SelectByMime(mimeType string) (DocumentParser, error) {
	for _, p := range s.parsers {
		if p.Supports(mimeType) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no parser supports mime type %q", mimeType)
}

// SelectByType 按 parser type 精确匹配。
func (s *DocumentParserSelector) SelectByType(t ParserType) (DocumentParser, error) {
	if p, ok := s.byType[t]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("no parser registered for type %q", t)
}

// Parse 是 selector 的便利方法：先按 MIME 选，再调 Parse
func (s *DocumentParserSelector) Parse(ctx context.Context, data []byte, mimeType, fileName string) (*ParseResult, error) {
	p, err := s.SelectByMime(mimeType)
	if err != nil {
		return nil, err
	}
	return p.Parse(ctx, data, mimeType, fileName)
}
