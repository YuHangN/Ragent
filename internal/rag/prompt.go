package rag

import (
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

const kbSystemPrompt = `你是一个专业的知识库助手。请严格根据以下检索到的内容回答用户问题，不要编造检索内容中未提及的信息。
如果检索内容中没有相关信息，请明确告知用户"根据当前知识库，没有找到相关内容"。`

// PromptService 根据检索结果构建发送给 LLM 的完整消息序列。
type PromptService struct{}

func NewPromptService() *PromptService { return &PromptService{} }

// BuildMessages 构建完整的消息序列：[System] + [KB上下文(User)] + [历史对话] + [用户问题(User)]。
func (s *PromptService) BuildMessages(question string, chunks []RetrievedChunk, history []aiclient.ChatMessage) []aiclient.ChatMessage {
	var messages []aiclient.ChatMessage

	// 1. System 提示词
	messages = append(messages, aiclient.System(kbSystemPrompt))

	// 2. 检索到的 KB 上下文
	if len(chunks) > 0 {
		context := formatContext(chunks)
		messages = append(messages, aiclient.User("以下是检索到的相关内容，请作为回答依据：\n\n"+context))
		messages = append(messages, aiclient.Assistant("好的，我已了解检索到的内容，请提问。"))
	}

	// 3. 历史对话
	messages = append(messages, history...)

	// 4. 用户当前问题
	messages = append(messages, aiclient.User(question))

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
