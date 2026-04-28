package aiclient

import (
	"strings"
)

type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderOllama Provider = "ollama"
	ProviderCohere Provider = "cohere"
	ProviderNoop   Provider = "noop"
)

// Matches 大小写无关比较
func (p Provider) Matches(s string) bool {
	return s != "" && strings.EqualFold(string(p), s)
}

func (p Provider) String() string { return string(p) }
