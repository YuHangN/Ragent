package fetcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalFileFetcher struct {
	allowedRoots []string // 必须是绝对路径
}

func NewLocalFileFetcher(allowedRoots []string) *LocalFileFetcher {
	abs := make([]string, 0, len(allowedRoots))
	for _, r := range allowedRoots {
		a, err := filepath.Abs(r)
		if err == nil {
			abs = append(abs, a)
		}
	}

	return &LocalFileFetcher{allowedRoots: abs}
}

func (LocalFileFetcher) Type() SourceType { return SourceLocal }

func (f LocalFileFetcher) Fetch(_ context.Context, req FetchRequest) (*FetchResult, error) {
	abs, err := filepath.Abs(req.Location)
	if err != nil {
		return nil, fmt.Errorf("local fetcher: abs path: %w", err)
	}
	if !f.isAllowed(abs) {
		return nil, fmt.Errorf("local fetcher: path %q not under any allowed root", abs)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("local fetcher: read %q: %w", abs, err)
	}

	fileName := req.FileName
	if fileName == "" {
		fileName = filepath.Base(abs)
	}
	return &FetchResult{Bytes: data, FileName: fileName}, nil
}

func (f LocalFileFetcher) isAllowed(abs string) bool {
	for _, root := range f.allowedRoots {
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
