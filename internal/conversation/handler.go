// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 本文件提供 HTTP 入口：会话 CRUD + 同步问答 + SSE 流式问答。所有路由都假定
// 调用方已经挂上 JWT 鉴权中间件（usercontext 已注入 LoginUser），handler 内
// 直接读 usercontext 拿 UserID 即可。
package conversation

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/internal/rag"
	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
)

// defaultMessageHistoryLimit 控制 GET /conversations/:id 返回的消息数。
//
// 200 条够日常会话展示；超长会话由前端做"加载更早消息"按钮调分页接口（MVP 不做）。
const defaultMessageHistoryLimit = 200

// Handler 是 chat 模块的 HTTP 入口聚合。
//
// 单一 Handler 同时承载 chat 调用与会话 CRUD，避免拆成两个 Handler 引起 main.go
// 装配重复；ConversationService 与 ChatService 拿到各自需要的依赖即可。
type Handler struct {
	chat *ChatService
	conv *ConversationService
	repo ConversationRepo // 直接持有 repo，给"列表 / 详情"这类纯读路径用
}

// NewHandler 构造 Handler。
//
// repo 故意单独注入而不是从 ConversationService 反射拿——service 是业务层入口，
// 不应该暴露内部 repo 给 handler；现在显式传两次，依赖关系更清晰。
func NewHandler(chat *ChatService, conv *ConversationService, repo ConversationRepo) *Handler {
	return &Handler{chat: chat, conv: conv, repo: repo}
}

// ──── 会话 CRUD ──────────────────────────────────────────

// CreateSession 创建一个新会话。POST /conversations
//
// 入参全部可选；title 为空时由首条 user 消息追加时自动回填。
// 返回 {id: "<snowflake>"}，前端后续 chat 请求带上即可。
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

// ListSessions 返回当前用户的会话列表。GET /conversations?limit=&offset=
//
// 按 update_time 倒序，与 ChatGPT 风格一致：最近活跃排最上。
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

// GetSession 返回会话基本信息 + 历史消息。GET /conversations/:id
//
// 消息按 create_time 升序返回，前端直接渲染聊天气泡。
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

// RenameSession 显式修改会话标题。PUT /conversations/:id
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

// DeleteSession 软删除一个会话。DELETE /conversations/:id
//
// 实际删除策略由 GORM 配置的 deleted 字段处理（软删）；消息表暂不级联删除，
// 关联消息可通过 deleted 字段恢复时仍然可见。
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

// Chat 同步问答。POST /chat
//
// 一次请求阻塞到 LLM 出完整答案。适合短答案场景或不需要流式 UI 的客户端。
// 流式答案走 ChatStream。
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

// ChatStream Server-Sent Events 流式问答。POST /chat/stream
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
// 客户端断连时 c.Request.Context() 自动 cancel，ChatService.StreamMessage
// 内部的 LLM 流也会随之停止，无需 handler 手动检测。
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

	// SSE 必要头：禁用各级代理 / 浏览器缓存。
	// X-Accel-Buffering: no 是给 nginx 看的，防止它默认 buffer 整段后再 flush。
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	cb := &sseCallback{w: c.Writer}
	// 错误已经通过 cb.OnError 推给前端；这里 err 仅用于服务端日志（middleware
	// 不会再处理，因为 response body 已经开始写 SSE 流，不能再回 JSON 错误）。
	_ = h.chat.StreamMessage(c.Request.Context(), SendRequest{
		ConversationID: convID,
		UserID:         uid,
		Question:       req.Question,
		KbIDs:          parseInt64Slice(req.KbIDs),
		TopK:           req.TopK,
	}, cb)
}

// requireUserID 从 gin.Context 取当前登录用户 ID 并解析成 int64。
//
// usercontext.Require 在未登录时 panic（由 Recovery middleware 转 401），
// 所以这里不会拿到 nil；UserID 解析失败返回 0——trace 归属用，0 表示"未知用户"，
// 不阻断 chat 主流程。
func requireUserID(c *gin.Context) int64 {
	user := usercontext.Require(c)
	uid, _ := strconv.ParseInt(user.UserID, 10, 64)
	return uid
}

// sseCallback 把 ChatService.StreamCallback 翻译成 SSE event。
//
// 写失败（客户端断连等）静默吞掉——SSE 流本来就是 best-effort 推送，
// 写失败时客户端已经走了，没必要再向上抛错。
type sseCallback struct {
	w gin.ResponseWriter
}

// OnDelta 推一条 chunk event 给前端，每个 LLM token delta 一次。
func (s *sseCallback) OnDelta(delta string) {
	writeSSE(s.w, "chunk", gin.H{"delta": delta})
}

// OnComplete 推 done 事件，携带完整答案 + 召回的 chunks。
//
// 前端可以直接拿完整 answer 作为校验（与累积 delta 对比），也可以渲染引用列表。
func (s *sseCallback) OnComplete(answer string, chunks []rag.RetrievedChunk) {
	writeSSE(s.w, "done", gin.H{
		"answer": answer,
		"chunks": toChunkDTOs(chunks),
	})
}

// OnError 推 error 事件；前端拿到后展示错误并停止接收。
func (s *sseCallback) OnError(err error) {
	writeSSE(s.w, "error", gin.H{"message": err.Error()})
}

// writeSSE 写一条 SSE event 并强制 flush。
//
// SSE 规范：每个 event 由 "event: <name>\ndata: <json>\n\n" 三部分组成，
// 空行表示一条 event 结束。flush 是必须的，否则数据卡在 Go HTTP buffer 里。
func writeSSE(w io.Writer, event string, payload any) {
	body, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
