// Package conversation 提供 RAG Chat 的会话管理与问答链路。
//
// 本文件定义 HTTP 层的请求和响应结构。对外 ID 使用 string，避免前端处理 int64
// 时发生精度丢失；召回片段通过独立 DTO 暴露，避免直接泄露内部检索模型。
package conversation

import (
	"strconv"
	"time"

	"github.com/YuHangN/ragent-go/internal/retrieval"
)

// ──── 请求 ──────────────────────────────────────────────

// SessionCreateRequest 是创建会话的请求参数。
//
// 字段均可选：title 为空时可由首条用户消息自动回填；kbIds 为空表示暂不限定
// 知识库范围。
type SessionCreateRequest struct {
	KbIDs []string `json:"kbIds"`
	Title string   `json:"title"`
}

// ChatRequestDTO 是同步 /chat 与流式 /chat/stream 共用的请求参数。
//
// ConversationID 在传输层使用 string，服务端解析为 int64；解析失败视为非法请求。
type ChatRequestDTO struct {
	ConversationID string   `json:"conversationId" binding:"required"`
	Question       string   `json:"question" binding:"required"`
	KbIDs          []string `json:"kbIds"`
	TopK           int      `json:"topK"`
}

// RenameSessionRequest 是 PUT /conversations/:id 的改名请求参数。
type RenameSessionRequest struct {
	Title string `json:"title" binding:"required"`
}

// ──── 响应 ──────────────────────────────────────────────

// SessionVO 是会话基本信息的响应结构。
//
// KbIDs 直接返回数据库中的 JSON 数组字符串，前端可按需解析；写入和读取保持
// 同一表示，避免在 DTO 中维护额外的 ID 切片字段。
type SessionVO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	KbIDs     string    `json:"kbIds"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MessageVO 是单条历史消息的响应结构。
//
// ChunksJSON 通常只在 assistant 消息上有值，前端可按需解析后渲染引用信息。
type MessageVO struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	ChunksJSON     string    `json:"chunksJson,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ChatResponseDTO 是同步 /chat 的响应结构。
//
// Chunks 与流式 done 事件保持同一结构，便于前端复用引用渲染逻辑。
type ChatResponseDTO struct {
	Answer string     `json:"answer"`
	Chunks []ChunkDTO `json:"chunks"`
}

// ChunkDTO 是 RAG 召回片段的响应结构。
//
// 仅暴露前端展示引用所需的字段，隐藏 collection、doc_id 等内部检索细节。
type ChunkDTO struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
	KbID    string  `json:"kbId"`
}

// ──── 转换辅助 ──────────────────────────────────────────

// toSessionVO 将会话模型转换为响应结构。
func toSessionVO(c *Conversation) SessionVO {
	return SessionVO{
		ID:        strconv.FormatInt(c.ID, 10),
		Title:     c.Title,
		KbIDs:     c.KbIDs,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// toSessionVOs 批量转换会话列表。
func toSessionVOs(list []Conversation) []SessionVO {
	out := make([]SessionVO, 0, len(list))
	for i := range list {
		out = append(out, toSessionVO(&list[i]))
	}
	return out
}

// toMessageVO 将消息模型转换为响应结构。
func toMessageVO(m *Message) MessageVO {
	return MessageVO{
		ID:             strconv.FormatInt(m.ID, 10),
		ConversationID: strconv.FormatInt(m.ConversationID, 10),
		Role:           string(m.Role),
		Content:        m.Content,
		ChunksJSON:     m.ChunksJSON,
		CreatedAt:      m.CreatedAt,
	}
}

// toMessageVOs 批量转换消息列表。
func toMessageVOs(list []Message) []MessageVO {
	out := make([]MessageVO, 0, len(list))
	for i := range list {
		out = append(out, toMessageVO(&list[i]))
	}
	return out
}

// toChunkDTOs 将检索召回片段转换为响应结构。
func toChunkDTOs(chunks []retrieval.RetrievedChunk) []ChunkDTO {
	out := make([]ChunkDTO, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, ChunkDTO{
			ID:      c.ID,
			Content: c.Content,
			Score:   c.Score,
			KbID:    strconv.FormatInt(c.KbID, 10),
		})
	}
	return out
}

// parseInt64Slice 将字符串 ID 切片解析为 int64 切片，无法解析的条目会被忽略。
//
// 该函数只做宽松的格式转换；知识库是否存在由后续检索链路处理。
func parseInt64Slice(strs []string) []int64 {
	out := make([]int64, 0, len(strs))
	for _, s := range strs {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}
