// Package admin 提供 RAG 系统的运维接口与链路追踪能力。
//
// 本文件负责注册 admin 路由。调用方负责在传入的 RouterGroup 上挂载鉴权、
// 授权等中间件。
package admin

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册 admin 模块路由。
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	traces := rg.Group("/admin/traces")
	traces.GET("", h.ListTraces)
	traces.GET("/:id", h.GetTrace)
}
