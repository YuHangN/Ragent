// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 本文件定义 HTTP 层使用的请求/响应 DTO。两类约定：
//   - 所有 ID（会话 ID、知识库 ID）在 wire 上用 string，避免 JavaScript 处理
//     int64 时的精度丢失（>2^53 时 number 类型会失去精度）。
//   - chunk 元信息独立成 chunkDTO 而不是直接暴露 rag.RetrievedChunk，方便后续
//     字段演进且不污染内部领域模型。
package conversation

import (
	"strconv"
	"time"

	"github.com/YuHangN/ragent-go/internal/rag"
)

// ──── 请求 ──────────────────────────────────────────────

// SessionCreateRequest 是创建会话的入参。
//
// 字段都可选：title 为空时由首条 user 消息自动回填；kbIds 为空表示"暂不指定
// 知识库范围"，后续 chat 请求可以再传。
type SessionCreateRequest struct {
	KbIDs []string `json:"kbIds"`
	Title string   `json:"title"`
}

// ChatRequestDTO 是同步 /chat 与流式 /chat/stream 共用入参。
//
// ConversationID 用 string 传，服务端解析为 int64；解析失败按非法请求处理，
// 不做 fallback。
type ChatRequestDTO struct {
	ConversationID string   `json:"conversationId" binding:"required"`
	Question       string   `json:"question" binding:"required"`
	KbIDs          []string `json:"kbIds"`
	TopK           int      `json:"topK"`
}

// RenameSessionRequest 是 PUT /conversations/:id 改名入参。
type RenameSessionRequest struct {
	Title string `json:"title" binding:"required"`
}

// ──── 响应 ──────────────────────────────────────────────

// SessionVO 是会话基本信息的对外形态。
//
// KbIDs 字段直接透传 DB 里的 JSON 数组字符串（例如 `[1,2,3]`），前端按需 JSON.parse。
// 不在服务端做反序列化是为了避免引入一个独立的 []string 字段——KbIDs 在写时
// 也是 JSON 序列化的，对称即可。
type SessionVO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	KbIDs     string    `json:"kbIds"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MessageVO 是单条历史消息的对外形态。
//
// ChunksJSON 仅在 role=assistant 时有值，原样透传给前端，前端按需 JSON.parse
// 来渲染"展开引用"。
type MessageVO struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	ChunksJSON     string    `json:"chunksJson,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ChatResponseDTO 是同步 /chat 的响应。
//
// Chunks 用 chunkDTO 列表暴露给前端，便于做引用 [1][2][3] 渲染；SSE 流式响应
// 在 done 事件里复用同样的 chunks 形状，保持前端两条路径解析逻辑一致。
type ChatResponseDTO struct {
	Answer string     `json:"answer"`
	Chunks []ChunkDTO `json:"chunks"`
}

// ChunkDTO 是 RAG 召回片段的对外形态。
//
// 故意不暴露 CollectionName 与 DocID——这两个偏运维细节，前端展示用不上；
// 真要审计请走 Phase 8 RagTrace 表。
type ChunkDTO struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
	KbID    string  `json:"kbId"`
}

// ──── 转换辅助 ──────────────────────────────────────────

// toSessionVO 把内部模型转 wire 形态。
func toSessionVO(c *Conversation) SessionVO {
	return SessionVO{
		ID:        strconv.FormatInt(c.ID, 10),
		Title:     c.Title,
		KbIDs:     c.KbIDs,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// toSessionVOs 批量转换。
func toSessionVOs(list []Conversation) []SessionVO {
	out := make([]SessionVO, 0, len(list))
	for i := range list {
		out = append(out, toSessionVO(&list[i]))
	}
	return out
}

// toMessageVO 把内部消息模型转 wire 形态。
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

// toMessageVOs 批量转换。
func toMessageVOs(list []Message) []MessageVO {
	out := make([]MessageVO, 0, len(list))
	for i := range list {
		out = append(out, toMessageVO(&list[i]))
	}
	return out
}

// toChunkDTOs 把 rag.RetrievedChunk 列表转 wire 形态。
func toChunkDTOs(chunks []rag.RetrievedChunk) []ChunkDTO {
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

// parseInt64Slice 把字符串数组解析成 int64 数组，无法解析的条目静默丢弃。
//
// 选择"丢弃 vs 整体 400"的权衡：调试场景前端传入个别脏数据时不希望整请求失败；
// 真正校验放在业务层（KbID 不存在时 RAGCoreService 会自然过滤掉）。
func parseInt64Slice(strs []string) []int64 {
	out := make([]int64, 0, len(strs))
	for _, s := range strs {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}
