package aiclient

import (
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

// ModelTarget 将模型候选和 provider 配置绑定为一次可调用目标。
type ModelTarget struct {
	ID        string
	Candidate config.ModelCandidate
	Provider  config.ProviderConfig
}

// NewModelTarget 构造 ModelTarget，并在候选 ID 为空时生成稳定兜底 ID。
func NewModelTarget(c config.ModelCandidate, p config.ProviderConfig) ModelTarget {
	id := c.ID
	if id == "" {
		id = fmt.Sprintf("%s::%s", coalesce(c.Provider, "unknown"), coalesce(c.Model, "unknown"))
	}

	return ModelTarget{ID: id, Candidate: c, Provider: p}
}

func coalesce(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
