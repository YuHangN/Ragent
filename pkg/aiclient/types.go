package aiclient

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

func System(content string) ChatMessage    { return ChatMessage{RoleSystem, content} }
func User(content string) ChatMessage      { return ChatMessage{RoleUser, content} }
func Assistant(content string) ChatMessage { return ChatMessage{RoleAssistant, content} }

// ChatRequest holds all parameters for a single LLM call.
type ChatRequest struct {
	Messages    []ChatMessage
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Thinking    bool // enable chain-of-thought if model supports it
	EnableTools bool // reserved for Phase 7
}

// StreamCallback receives incremental output from a streaming LLM call.
type StreamCallback interface {
	OnContent(delta string)
	OnThinking(delta string)
	OnComplete()
	OnError(err error)
}

// RetrievedChunk is a chunk returned from vector search or after reranking.
type RetrievedChunk struct {
	ID    string
	Text  string
	Score float32
}
