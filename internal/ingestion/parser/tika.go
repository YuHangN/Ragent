package parser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TikaDocumentParser struct {
	url     string
	timeout time.Duration
	client  *http.Client
}

func NewTikaDocumentParser(url string, timeout time.Duration) *TikaDocumentParser {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &TikaDocumentParser{
		url:     url,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (TikaDocumentParser) Type() ParserType { return ParserTypeTika }

func (TikaDocumentParser) Supports(mimeType string) bool {
	lower := strings.ToLower(mimeType)
	binaryHints := []string{"pdf", "msword", "wordprocessingml", "excel", "spreadsheetml",
		"powerpoint", "presentationml", "rtf", "epub"}
	for _, h := range binaryHints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

func (p TikaDocumentParser) Parse(ctx context.Context, data []byte, mimeType string, fileName string) (*ParseResult, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, p.url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("tika: build request: %w", err)
	}
	req.Header.Set("Accept", "text/plain")
	if mimeType != "" {
		req.Header.Set("Content-Type", mimeType)
	}
	if fileName != "" {
		req.Header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tika: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tika: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tika: status %d: %s", resp.StatusCode, string(body))
	}

	return &ParseResult{
		Text: CleanupText(string(body)),
		Metadata: map[string]string{
			"tika.contentType": resp.Header.Get("Content-Type"),
		},
	}, nil
}
