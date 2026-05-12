// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 本文件负责把 chat + 会话 CRUD 的 HTTP 路由注册到上层 RouterGroup。
// 不在这里挂鉴权中间件——调用方决定整组路由用什么 middleware（与 knowledge
// 模块同样做法，鉴权姿势在 server/router.go 集中可见）。
package conversation

import "github.com/gin-gonic/gin"

// RegisterRoutes 把 RAG Chat 的全部路由注册到给定的 RouterGroup。
//
// 路由分两类：
//   - /conversations 下挂会话 CRUD
//   - /chat 与 /chat/stream 是问答主入口；流式版本走 SSE
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	conv := rg.Group("/conversations")
	conv.POST("", h.CreateSession)
	conv.GET("", h.ListSessions)
	conv.GET("/:id", h.GetSession)
	conv.PUT("/:id", h.RenameSession)
	conv.DELETE("/:id", h.DeleteSession)

	rg.POST("/chat", h.Chat)
	rg.POST("/chat/stream", h.ChatStream)
}
