package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

type HttpUrlFetcher struct {
	client       *http.Client
	timeout      time.Duration
	maxBodyBytes int64
}

func NewHttpUrlFetcher(timeout time.Duration, maxBodyBytes int64) *HttpUrlFetcher {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = 50 * 1024 * 1024
	}
	return &HttpUrlFetcher{
		client:       &http.Client{Timeout: timeout},
		timeout:      timeout,
		maxBodyBytes: maxBodyBytes,
	}
}

func (HttpUrlFetcher) Type() SourceType { return SourceHTTP }

func (f HttpUrlFetcher) Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error) {
	if !strings.HasPrefix(req.Location, "http://") && !strings.HasPrefix(req.Location, "https://") {
		return nil, fmt.Errorf("http fetcher: invalid url %q", req.Location)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.Location, nil)
	if err != nil {
		return nil, fmt.Errorf("http fetcher: build request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "ragent-go/1.0")

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http fetcher: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http fetcher: status %d for %q", resp.StatusCode, req.Location)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("http fetcher: read body: %w", err)
	}
	if int64(len(data)) > f.maxBodyBytes {
		return nil, fmt.Errorf("http fetcher: body exceeds %d bytes", f.maxBodyBytes)
	}

	fileName := req.FileName
	if fileName == "" {
		fileName = path.Base(req.Location)
		if i := strings.IndexByte(fileName, '?'); i >= 0 {
			fileName = fileName[:i]
		}
	}

	return &FetchResult{
		Bytes:    data,
		MimeType: resp.Header.Get("Content-Type"),
		FileName: fileName,
	}, nil
}
