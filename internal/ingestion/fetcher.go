package ingestion

import (
	"context"
	"fmt"

	"github.com/YuHangN/ragent-go/internal/ingestion/fetcher"
	"github.com/YuHangN/ragent-go/internal/ingestion/parser"
)

type FetcherNode struct {
	s3 *fetcher.S3Fetcher
}

func NewFetcherNode(s3 *fetcher.S3Fetcher) *FetcherNode {
	return &FetcherNode{s3: s3}
}

func (n *FetcherNode) Name() string { return "fetcher" }

func (n *FetcherNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	// 幂等：raw bytes 已注入则跳过。
	if len(ic.RawBytes) > 0 {
		if ic.MimeType == "" && ic.Source != nil {
			ic.MimeType = parser.DetectMimeType(ic.RawBytes, ic.Source.FileName)
		}
		return OK(fmt.Sprintf("skipped — raw bytes already present (%d bytes)", len(ic.RawBytes)))
	}

	if ic.Source == nil {
		return Fail(fmt.Errorf("fetcher: source is nil"))
	}

	res, err := n.s3.Fetch(ctx, fetcher.FetchRequest{
		Type:     fetcher.SourceS3,
		Location: ic.Source.Location, // 必须是 s3://bucket/key
		FileName: ic.Source.FileName,
	})
	if err != nil {
		return Fail(fmt.Errorf("fetcher: %w", err))
	}

	ic.RawBytes = res.Bytes
	if res.FileName != "" {
		ic.Source.FileName = res.FileName
	}
	if res.MimeType != "" {
		ic.MimeType = res.MimeType
	} else {
		ic.MimeType = parser.DetectMimeType(res.Bytes, ic.Source.FileName)
	}

	return OK(fmt.Sprintf("fetched %d bytes from %s (%s)", len(res.Bytes), ic.Source.Location, ic.MimeType))
}