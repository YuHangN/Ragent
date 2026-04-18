package ingestion

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// FetcherNode downloads raw document bytes from S3.
// If RawBytes is already populated it skips the download (idempotency).
type FetcherNode struct {
	s3Client *s3.Client
}

func NewFetcherNode(s3Client *s3.Client) *FetcherNode {
	return &FetcherNode{s3Client: s3Client}
}

func (n *FetcherNode) Name() string { return "fetcher" }

func (n *FetcherNode) Execute(ctx context.Context, ic *IngestionContext) NodeResult {
	if len(ic.RawBytes) > 0 {
		if ic.MimeType == "" {
			ic.MimeType = detectMimeType(ic.Source)
		}
		return OK(fmt.Sprintf("skipped — raw bytes already present (%d bytes)", len(ic.RawBytes)))
	}

	if ic.Source == nil {
		return Fail(fmt.Errorf("fetcher: document source is nil"))
	}

	switch ic.Source.Type {
	case SourceTypeS3:
		return n.fetchS3(ctx, ic)
	case SourceTypeRaw:
		return Fail(fmt.Errorf("fetcher: source type 'raw' requires pre-populated RawBytes"))
	default:
		return Fail(fmt.Errorf("fetcher: unsupported source type %q", ic.Source.Type))
	}
}

// fetchS3 reads the object at ic.Source.Location (e.g. "s3://kb-123/doc.pdf").
func (n *FetcherNode) fetchS3(ctx context.Context, ic *IngestionContext) NodeResult {
	loc := ic.Source.Location
	if !strings.HasPrefix(loc, "s3://") {
		return Fail(fmt.Errorf("fetcher: invalid S3 location %q (must start with s3://)", loc))
	}

	rest := strings.TrimPrefix(loc, "s3://")
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return Fail(fmt.Errorf("fetcher: cannot parse bucket/key from %q", loc))
	}

	bucket := rest[:slash]
	key := rest[slash+1:]

	out, err := n.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(fmt.Errorf("fetcher: S3 GetObject %q: %w", loc, err))
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return Fail(fmt.Errorf("fetcher: read S3 body: %w", err))
	}
	ic.RawBytes = data

	if ic.Source.FileName == "" {
		ic.Source.FileName = filepath.Base(key)
	}
	ic.MimeType = detectMimeType(ic.Source)

	return OK(fmt.Sprintf("fetched %d bytes from %s", len(data), loc))
}

func detectMimeType(src *DocumentSource) string {
	if src == nil || src.FileName == "" {
		return "application/octet-stream"
	}

	switch strings.ToLower(filepath.Ext(src.FileName)) {
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".pdf":
		return "application/pdf"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}
