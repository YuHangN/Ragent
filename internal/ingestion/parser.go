package ingestion

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

type ParserNode struct{}

func NewParserNode() *ParserNode { return &ParserNode{} }

func (n *ParserNode) Name() string { return "parser" }

func (n *ParserNode) Execute(_ context.Context, ic *IngestionContext) NodeResult {
	if len(ic.RawBytes) == 0 {
		return Fail(fmt.Errorf("parser: no raw bytes to parse"))
	}

	text, err := parseBytes(ic.RawBytes, ic.MimeType, ic.Source)
	if err != nil {
		return Fail(fmt.Errorf("parser: %w", err))
	}

	ic.RawText = text
	return OK(fmt.Sprintf("parsed %d chars from %s", len([]rune(text)), ic.MimeType))
}

func parseBytes(data []byte, mimeType string, src *DocumentSource) (string, error) {
	lower := strings.ToLower(mimeType)

	// Text and markdown — just decode as UTF-8.
	if strings.HasPrefix(lower, "text/") {
		return sanitizeText(string(data)), nil
	}

	// JSON — treat as text.
	if lower == "application/json" {
		return sanitizeText(string(data)), nil
	}

	// HTML — strip tags for a best-effort plain text.
	if strings.Contains(lower, "html") {
		return stripHTMLTags(string(data)), nil
	}

	// PDF / Word / Excel — not yet supported.
	// Return an error describing what the user should do.
	if strings.Contains(lower, "pdf") || strings.Contains(lower, "word") ||
		strings.Contains(lower, "excel") || strings.Contains(lower, "sheet") {
		return "", fmt.Errorf("file type %q not yet supported — only text/markdown files are parsed in Phase 5", mimeType)
	}

	// Unknown binary — attempt UTF-8 decode, return error if not valid text.
	s := string(data)
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("file type %q is binary and not valid UTF-8", mimeType)
	}

	return sanitizeText(s), nil
}

func sanitizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// stripHTMLTags does a simple < > removal — not production-grade but avoids
func stripHTMLTags(html string) string {
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
	return sanitizeText(b.String())
}
