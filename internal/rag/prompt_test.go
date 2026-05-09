package rag

import (
	"testing"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptService_BuildMessages_WithChunks(t *testing.T) {
	svc := NewPromptService()
	chunks := []RetrievedChunk{
		{ID: "1", Content: "RAG 是检索增强生成技术"},
		{ID: "2", Content: "向量数据库用于存储 embedding"},
	}
	history := []aiclient.ChatMessage{
		aiclient.User("上一个问题"),
		aiclient.Assistant("上一个回答"),
	}

	messages := svc.BuildMessages("什么是 RAG？", chunks, history)

	// 期望顺序：System + KB上下文(User) + KB确认(Assistant) + 历史*2 + 用户问题
	require.Len(t, messages, 6)
	assert.Equal(t, aiclient.RoleSystem, messages[0].Role)
	assert.Equal(t, aiclient.RoleUser, messages[1].Role) // KB 上下文
	assert.Contains(t, messages[1].Content, "RAG 是检索增强生成技术")
	assert.Equal(t, aiclient.RoleUser, messages[5].Role) // 用户问题
	assert.Equal(t, "什么是 RAG？", messages[5].Content)
}

func TestPromptService_BuildMessages_NoChunks(t *testing.T) {
	svc := NewPromptService()
	messages := svc.BuildMessages("什么是 RAG？", nil, nil)

	// 无 chunks：System + 用户问题
	require.Len(t, messages, 2)
	assert.Equal(t, aiclient.RoleSystem, messages[0].Role)
	assert.Equal(t, "什么是 RAG？", messages[1].Content)
}

func TestFormatContext(t *testing.T) {
	chunks := []RetrievedChunk{
		{Content: "内容A"},
		{Content: "内容B"},
	}
	result := formatContext(chunks)
	assert.Contains(t, result, "[1] 内容A")
	assert.Contains(t, result, "[2] 内容B")
}
