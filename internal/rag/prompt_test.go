package rag

import (
	"testing"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptService_BuildMessages_WithChunks(t *testing.T) {
	svc := NewPromptService()
	pc := PromptContext{
		Scene:    PromptSceneKBOnly,
		Question: "什么是 RAG？",
		Chunks: []RetrievedChunk{
			{ID: "1", Content: "RAG 是检索增强生成技术"},
		},
		History: []aiclient.ChatMessage{
			aiclient.User("上一个问题"),
			aiclient.Assistant("上一个回答"),
		},
	}

	messages := svc.BuildMessages(pc)
	require.Len(t, messages, 6)
	assert.Equal(t, aiclient.RoleSystem, messages[0].Role)
	assert.Contains(t, messages[1].Content, "RAG 是检索增强生成技术")
}

func TestPromptService_BuildMessages_KBOnly_NoChunks(t *testing.T) {
	svc := NewPromptService()
	pc := PromptContext{Scene: PromptSceneKBOnly, Question: "什么是 RAG？"}
	messages := svc.BuildMessages(pc)
	require.Len(t, messages, 2)
}

func TestPromptService_BuildMessages_SystemOnly_ReturnsNil(t *testing.T) {
	svc := NewPromptService()
	pc := PromptContext{Scene: PromptSceneSystemOnly, Question: "你好"}
	assert.Nil(t, svc.BuildMessages(pc))
}
