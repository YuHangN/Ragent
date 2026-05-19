package retrieval

import (
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

const kbSystemPrompt = `你是一个专业的知识库助手。请严格根据以下检索到的内容回答用户问题，不要编造检索内容中未提及的信息。
如果检索内容中没有相关信息，请明确告知用户"根据当前知识库，没有找到相关内容"。`

// PromptScene 描述 Prompt 构建的场景
type PromptScene string

const (
	PromptSceneKBOnly     PromptScene = "KB_ONLY"     // Phase 6 默认
	PromptSceneMCPOnly    PromptScene = "MCP_ONLY"    // Phase 10 启用
	PromptSceneMixed      PromptScene = "MIXED"       // Phase 10 启用
	PromptSceneSystemOnly PromptScene = "SYSTEM_ONLY" // 短路，不调 LLM
)

// PromptContext 是 PromptService 的完整入参。
type PromptContext struct {
	Scene    PromptScene
	Question string
	Chunks   []RetrievedChunk
	History  []aiclient.ChatMessage
	// 预留字段：Phase 7+ 启用
	McpResults  []string // MCP 工具返回结果文本（MIXED / MCP_ONLY 用）
	SystemReply string   // SYSTEM_ONLY 时直接返回这条文本（不进 LLM）
}

// PromptService 根据检索结果构建发送给 LLM 的完整消息序列。
type PromptService struct{}

func NewPromptService() *PromptService { return &PromptService{} }

// BuildMessages 按 Scene 路由到对应实现
func (s *PromptService) BuildMessages(pc PromptContext) []aiclient.ChatMessage {
	switch pc.Scene {
	case PromptSceneMCPOnly, PromptSceneMixed:
		// Phase 10 启用：先返回 KB_ONLY 行为占位
		return s.buildKBOnly(pc)
	case PromptSceneSystemOnly:
		// 不应调用到这里（RAGCoreService 应已短路）。兜底返回空。
		return nil
	default:
		return s.buildKBOnly(pc)
	}
}

// buildKBOnly 构建完整的消息序列：[System] + [KB上下文(User)] + [历史对话] + [用户问题(User)]。
func (s *PromptService) buildKBOnly(pc PromptContext) []aiclient.ChatMessage {
	var messages []aiclient.ChatMessage

	// 1. System 提示词
	messages = append(messages, aiclient.System(kbSystemPrompt))

	// 2. 检索到的 KB 上下文
	if len(pc.Chunks) > 0 {
		context := formatContext(pc.Chunks)
		messages = append(messages, aiclient.User("以下是检索到的相关内容，请作为回答依据：\n\n"+context))
		messages = append(messages, aiclient.Assistant("好的，我已了解检索到的内容，请提问。"))
	}

	// 3. 历史对话
	messages = append(messages, pc.History...)

	// 4. 用户当前问题
	messages = append(messages, aiclient.User(pc.Question))

	return messages
}

// formatContext 将 chunks 格式化为带编号的参考文本块。
func formatContext(chunks []RetrievedChunk) string {
	var sb strings.Builder
	for i, chunk := range chunks {
		sb.WriteString(fmt.Sprintf("[%d] %s\n\n", i+1, chunk.Content))
	}
	return strings.TrimSpace(sb.String())
}
