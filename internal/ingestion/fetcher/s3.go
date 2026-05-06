package fetcher

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Fetcher struct {
	client *s3.Client
}

func NewS3Fetcher(client *s3.Client) *S3Fetcher { return &S3Fetcher{client: client} }

func (S3Fetcher) Type() SourceType { return SourceS3 }

func (f S3Fetcher) Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error) {
	loc := req.Location
	if !strings.HasPrefix(loc, "s3://") {
		return nil, fmt.Errorf("s3 fetcher: invalid location %q (must start with s3://)", loc)
	}
	rest := strings.TrimPrefix(loc, "s3://")
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return nil, fmt.Errorf("s3 fetcher: cannot parse bucket/key from %q", loc)
	}

	bucket, key := rest[:slash], rest[slash+1:]

	out, err := f.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 fetcher: GetObject %q: %w", loc, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("s3 fetcher: read body: %w", err)
	}

	fileName := req.FileName
	if fileName == "" {
		fileName = filepath.Base(key)
	}
	mime := ""
	if out.ContentType != nil {
		mime = *out.ContentType
	}

	return &FetchResult{Bytes: data, MimeType: mime, FileName: fileName}, nil
}
