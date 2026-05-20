package aiclient

import (
	"strings"
)

type Provider string

const (
	// ProviderOpenAI 表示 OpenAI 或 OpenAI 兼容的远程 API。
	ProviderOpenAI Provider = "openai"
	// ProviderOllama 表示本地或远程 Ollama 兼容服务。
	ProviderOllama Provider = "ollama"
	// ProviderCohere 表示 Cohere 重排兼容 API。
	ProviderCohere Provider = "cohere"
	// ProviderNoop 表示内置的空操作 provider，通常用于兜底。
	ProviderNoop Provider = "noop"
)

// Matches 判断 s 是否表示同一个 provider，比较时忽略大小写。
func (p Provider) Matches(s string) bool {
	return s != "" && strings.EqualFold(string(p), s)
}

// String 返回配置中使用的 provider 名称。
func (p Provider) String() string { return string(p) }
