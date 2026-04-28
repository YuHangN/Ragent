package aiclient

import (
	"fmt"

	"github.com/YuHangN/ragent-go/config"
)

// ModelTarget 把候选和 provider 绑成一个可调用单元
type ModelTarget struct {
	ID        string
	Candidate config.ModelCandidate
	Provider  config.ProviderConfig
}

// NewModelTarget 构造 ModelTarget。Candidate.ID 为空时用 "provider::model" fallback。
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
