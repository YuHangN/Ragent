package aiclient

type Role string

const (
	// RoleSystem 表示用于定义助手行为的系统消息。
	RoleSystem Role = "system"
	// RoleUser 表示用户输入的消息。
	RoleUser Role = "user"
	// RoleAssistant 表示助手已经生成的消息。
	RoleAssistant Role = "assistant"
)

// ChatMessage 表示聊天上下文中的一条消息。
type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// System 构造一条 system 角色消息。
func System(content string) ChatMessage { return ChatMessage{RoleSystem, content} }

// User 构造一条 user 角色消息。
func User(content string) ChatMessage { return ChatMessage{RoleUser, content} }

// Assistant 构造一条 assistant 角色消息。
func Assistant(content string) ChatMessage { return ChatMessage{RoleAssistant, content} }

// ChatRequest 描述一次聊天补全调用所需的全部参数。
type ChatRequest struct {
	Messages    []ChatMessage
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Thinking    bool // 要求 selector 优先选择支持思考模式的模型。
	EnableTools bool // 预留给工具调用聊天流程。
}

// StreamCallback 接收流式聊天调用中的增量输出。
type StreamCallback interface {
	OnContent(delta string)
	OnThinking(delta string)
	OnComplete()
	OnError(err error)
}

// RetrievedChunk 表示向量检索返回的片段，也可携带重排后的分数。
type RetrievedChunk struct {
	ID    string
	Text  string
	Score float32
}
