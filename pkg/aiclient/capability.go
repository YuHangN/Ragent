package aiclient

type Capability string

const (
	CapabilityChat      Capability = "chat"
	CapabilityEmbedding Capability = "embedding"
	CapabilityRerank    Capability = "rerank"
)

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

func (c Capability) String() string { return string(c) }
