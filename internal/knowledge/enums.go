package knowledge

import (
	"strings"

	"github.com/YuHangN/ragent-go/pkg/apperror"
)

type DocumentStatus string

const (
	DocStatusPending DocumentStatus = "pending"
	DocStatusRunning DocumentStatus = "running"
	DocStatusSuccess DocumentStatus = "success"
	DocStatusFailed  DocumentStatus = "failed"
)

func (s DocumentStatus) String() string { return string(s) }

// SourceType 文档来源类型
type SourceType string

const (
	SourceTypeFile SourceType = "file"
	SourceTypeURL  SourceType = "url"
)

func (s SourceType) String() string { return string(s) }

// NormalizeSourceType 规范化来源类型。
func NormalizeSourceType(raw string) (SourceType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "file", "localfile", "local_file":
		return SourceTypeFile, nil
	case "url":
		return SourceTypeURL, nil
	case "":
		return SourceTypeFile, nil
	default:
		return "", apperror.NewClientMsg("来源类型不合法：" + raw)
	}
}

// ProcessMode 文档处理模式
type ProcessMode string

const (
	ProcessModeChunk    ProcessMode = "chunk"
	ProcessModePipeline ProcessMode = "pipeline"
)

func (p ProcessMode) String() string { return string(p) }

// NormalizeProcessMode 规范化处理模式。空串默认 chunk。
func NormalizeProcessMode(raw string) (ProcessMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "chunk":
		return ProcessModeChunk, nil
	case "pipeline":
		return ProcessModePipeline, nil
	case "":
		return ProcessModeChunk, nil
	default:
		return "", apperror.NewClientMsg("处理模式不合法：" + raw)
	}
}
