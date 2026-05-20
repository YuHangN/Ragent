package aiclient

type Capability string

const (
	// CapabilityChat 表示聊天补全能力。
	CapabilityChat Capability = "chat"
	// CapabilityEmbedding 表示文本向量化能力。
	CapabilityEmbedding Capability = "embedding"
	// CapabilityRerank 表示文档重排能力。
	CapabilityRerank Capability = "rerank"
)

// DisplayName 返回适合日志和错误信息展示的能力名称。
func (c Capability) DisplayName() string {
	switch c {
	case CapabilityChat:
		return "Chat"
	case CapabilityEmbedding:
		return "Embedding"
	case CapabilityRerank:
		return "Rerank"
	default:
		return string(c)
	}
}

// String 返回能力在配置中的 key。
func (c Capability) String() string { return string(c) }
