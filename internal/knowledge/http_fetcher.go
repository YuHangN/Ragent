package knowledge

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/YuHangN/ragent-go/pkg/apperror"
)

// HTTPFetcher 对齐 Java HttpClientHelper，做 URL HEAD/GET + 文件名/ETag 提取。
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher 创建默认的 fetcher，超时 60s。
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// HeadResult HEAD 请求的结果。
type HeadResult struct {
	ETag          string
	LastModified  string
	ContentLength int64
	ContentType   string
}

// Head 发起 HEAD 请求读取元信息；某些服务器不支持 HEAD 会返回错误。
func (f *HTTPFetcher) Head(url string) (*HeadResult, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, apperror.NewRemoteWrap("构造 HEAD 请求失败", err, nil)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, apperror.NewRemoteWrap("HEAD 请求失败", err, nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, apperror.NewRemoteMsgCode(fmt.Sprintf("HEAD 返回状态 %d", resp.StatusCode), nil)
	}

	return &HeadResult{
		ETag:          resp.Header.Get("ETag"),
		LastModified:  resp.Header.Get("Last-Modified"),
		ContentLength: resp.ContentLength,
		ContentType:   resp.Header.Get("Content-Type"),
	}, nil
}

// GetResult GET 请求的结果。
type GetResult struct {
	Body         []byte
	ContentType  string
	FileName     string
	ETag         string
	LastModified string
	ContentHash  string
}

// Get 发起 GET 请求，带 size 上限防止下载超大文件。
func (f *HTTPFetcher) Get(url string, maxSize int64) (*GetResult, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, apperror.NewRemoteWrap("构造 GET 请求失败", err, nil)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, apperror.NewRemoteWrap("GET 请求失败", err, nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, apperror.NewRemoteMsgCode(fmt.Sprintf("GET 返回状态 %d", resp.StatusCode), nil)
	}

	var reader io.Reader = resp.Body
	if maxSize > 0 {
		// 读 maxSize+1，超限时报错
		reader = io.LimitReader(resp.Body, maxSize+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, apperror.NewRemoteWrap("读取响应 body 失败", err, nil)
	}
	if maxSize > 0 && int64(len(body)) > maxSize {
		return nil, apperror.NewClientMsg(fmt.Sprintf("远程文件大小超过限制: %d bytes", maxSize))
	}
	if len(body) == 0 {
		return nil, apperror.NewClientMsg("远程文件内容为空")
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(body))
	return &GetResult{
		Body:         body,
		ContentType:  resp.Header.Get("Content-Type"),
		FileName:     extractFilename(resp.Header.Get("Content-Disposition"), url),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		ContentHash:  hash,
	}, nil
}

// extractFilename 从 Content-Disposition 提取 filename；失败时从 URL 末尾推断。
func extractFilename(contentDisposition, url string) string {
	if contentDisposition != "" {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			if name, ok := params["filename"]; ok && name != "" {
				return name
			}
		}
	}

	// URL 兜底：取最后一段 path
	idx := strings.LastIndex(url, "/")
	if idx >= 0 && idx < len(url)-1 {
		name := url[idx+1:]
		if q := strings.Index(name, "?"); q >= 0 {
			name = name[:q]
		}
		return name
	}
	return ""
}
