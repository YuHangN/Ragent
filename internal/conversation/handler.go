// Package conversation 提供 RAG Chat 的会话管理与问答链路。
//
// 本文件提供 HTTP 入口，包括会话管理、同步问答和 SSE 流式问答。路由假定调用方
// 已完成登录态注入，handler 通过 usercontext 读取当前用户。
package conversation

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/internal/retrieval"
	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
)

// defaultMessageHistoryLimit 是 GET /conversations/:id 返回的默认消息数。
//
// 该值用于限制详情接口一次返回的历史消息数量。
const defaultMessageHistoryLimit = 200

// Handler 是 conversation 模块的 HTTP 处理器。
//
// 它同时承载会话管理和问答入口，依赖 ConversationService、ChatService 以及
// 只读查询所需的 ConversationRepo。
type Handler struct {
	chat *ChatService
	conv *ConversationService
	repo ConversationRepo // 直接持有 repo，给"列表 / 详情"这类纯读路径用
}

// NewHandler 创建 Handler。
//
// repo 单独注入给列表和详情等纯读路径使用，避免 Handler 反向依赖
// ConversationService 的内部实现。
func NewHandler(chat *ChatService, conv *ConversationService, repo ConversationRepo) *Handler {
	return &Handler{chat: chat, conv: conv, repo: repo}
}

// ──── 会话 CRUD ──────────────────────────────────────────

// CreateSession 处理 POST /conversations，创建一个新会话。
//
// 请求字段均可选；title 为空时可由首条用户消息自动回填。
func (h *Handler) CreateSession(c *gin.Context) {
	user := usercontext.Require(c)
	uid, err := strconv.ParseInt(user.UserID, 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("登录用户 ID 非法"))
		return
	}

	var req SessionCreateRequest
	_ = c.ShouldBindJSON(&req) // 全字段可选，绑定失败按空请求处理

	conv, err := h.conv.CreateSession(uid, parseInt64Slice(req.KbIDs), req.Title)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"id": strconv.FormatInt(conv.ID, 10)}))
}

// ListSessions 处理 GET /conversations?limit=&offset=，返回当前用户的会话列表。
//
// 结果按 update_time 倒序排列，最近活跃的会话在前。
func (h *Handler) ListSessions(c *gin.Context) {
	user := usercontext.Require(c)
	uid, err := strconv.ParseInt(user.UserID, 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("登录用户 ID 非法"))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	list, total, err := h.repo.ListConversationsByUser(uid, limit, offset)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{
		"total": total,
		"list":  toSessionVOs(list),
	}))
}

// GetSession 处理 GET /conversations/:id，返回会话基本信息和历史消息。
//
// 消息按 create_time 升序返回，便于前端直接渲染聊天记录。
func (h *Handler) GetSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("会话 ID 格式错误"))
		return
	}
	conv, err := h.repo.FindConversationByID(id)
	if err != nil {
		_ = c.Error(err)
		return
	}
	msgs, err := h.repo.ListMessages(id, defaultMessageHistoryLimit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{
		"conversation": toSessionVO(conv),
		"messages":     toMessageVOs(msgs),
	}))
}

// RenameSession 处理 PUT /conversations/:id，修改会话标题。
func (h *Handler) RenameSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("会话 ID 格式错误"))
		return
	}
	var req RenameSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.conv.RenameTitle(id, req.Title); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// DeleteSession 处理 DELETE /conversations/:id，软删除一个会话。
//
// 会话删除由 GORM deleted 字段实现；消息表不在这里级联删除。
func (h *Handler) DeleteSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("会话 ID 格式错误"))
		return
	}
	if err := h.repo.DeleteConversation(id); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// ──── 同步 chat ──────────────────────────────────────────

// Chat 处理 POST /chat，执行同步问答。
//
// 请求会阻塞到 LLM 返回完整答案；需要增量输出的客户端使用 ChatStream。
func (h *Handler) Chat(c *gin.Context) {
	uid := requireUserID(c)
	var req ChatRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	convID, err := strconv.ParseInt(req.ConversationID, 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("会话 ID 格式错误"))
		return
	}

	resp, err := h.chat.SendMessage(c.Request.Context(), SendRequest{
		ConversationID: convID,
		UserID:         uid,
		Question:       req.Question,
		KbIDs:          parseInt64Slice(req.KbIDs),
		TopK:           req.TopK,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(ChatResponseDTO{
		Answer: resp.Answer,
		Chunks: toChunkDTOs(resp.Chunks),
	}))
}

// ──── 流式 chat ──────────────────────────────────────────

// ChatStream 处理 POST /chat/stream，通过 Server-Sent Events 返回流式答案。
//
// 事件格式：
//
//	event: chunk
//	data: {"delta": "部分..."}
//
//	event: done
//	data: {"answer": "完整", "chunks": [...]}
//
//	event: error
//	data: {"message": "..."}
//
// 客户端断连时请求 Context 会取消，底层流式调用会随之停止。
func (h *Handler) ChatStream(c *gin.Context) {
	uid := requireUserID(c)
	var req ChatRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	convID, err := strconv.ParseInt(req.ConversationID, 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("会话 ID 格式错误"))
		return
	}

	// SSE 头用于禁用代理和浏览器缓存；X-Accel-Buffering 用于关闭 nginx 缓冲。
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	cb := &sseCallback{w: c.Writer}
	// 错误已通过 cb.OnError 推给前端；响应体开始写入后不能再返回 JSON 错误。
	_ = h.chat.StreamMessage(c.Request.Context(), SendRequest{
		ConversationID: convID,
		UserID:         uid,
		Question:       req.Question,
		KbIDs:          parseInt64Slice(req.KbIDs),
		TopK:           req.TopK,
	}, cb)
}

// requireUserID 从 gin.Context 读取当前登录用户 ID 并解析为 int64。
//
// UserID 解析失败时返回 0，用于表示 trace 中未知用户，不阻断 chat 主流程。
func requireUserID(c *gin.Context) int64 {
	user := usercontext.Require(c)
	uid, _ := strconv.ParseInt(user.UserID, 10, 64)
	return uid
}

// sseCallback 将 ChatService.StreamCallback 转换为 SSE event。
//
// 写失败通常表示客户端已断开，回调会忽略该错误。
type sseCallback struct {
	w gin.ResponseWriter
}

// OnDelta 向前端发送一条 chunk event。
func (s *sseCallback) OnDelta(delta string) {
	writeSSE(s.w, "chunk", gin.H{"delta": delta})
}

// OnComplete 发送 done event，携带完整答案和召回片段。
//
// 前端可用完整 answer 校验增量内容，也可用 chunks 渲染引用列表。
func (s *sseCallback) OnComplete(answer string, chunks []retrieval.RetrievedChunk) {
	writeSSE(s.w, "done", gin.H{
		"answer": answer,
		"chunks": toChunkDTOs(chunks),
	})
}

// OnError 发送 error event，前端收到后可展示错误并停止接收。
func (s *sseCallback) OnError(err error) {
	writeSSE(s.w, "error", gin.H{"message": err.Error()})
}

// writeSSE 写入一条 SSE event 并尽量 flush。
//
// 每个 event 由 event 名、JSON data 和空行组成；flush 可降低流式输出延迟。
func writeSSE(w io.Writer, event string, payload any) {
	body, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
