// Package conversation 提供 RAG Chat 的会话管理与问答链路。
//
// 本文件负责将 chat 与会话管理路由注册到上层 RouterGroup。鉴权、授权等中间件
// 由调用方在路由组上统一挂载。
package conversation

import "github.com/gin-gonic/gin"

// RegisterRoutes 将 RAG Chat 的全部路由注册到给定的 RouterGroup。
//
// 路由分为两类：
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
