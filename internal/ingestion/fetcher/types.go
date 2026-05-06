package fetcher

import (
	"context"
)

type SourceType string

const (
	SourceS3     SourceType = "s3"
	SourceLocal  SourceType = "local"
	SourceHTTP   SourceType = "http"
	SourceFeishu SourceType = "feishu"
)

type FetchRequest struct {
	Type     SourceType
	Location string
	FileName string // 可选，无则由 fetcher 从 Location 推导
}

// FetchResult 抓取结果。
type FetchResult struct {
	Bytes    []byte
	MimeType string // 可选，能给就给（如 HTTP Content-Type 头）
	FileName string
}

// DocumentFetcher 把 FetchRequest 转成原始字节。
type DocumentFetcher interface {
	Type() SourceType
	Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error)
}
