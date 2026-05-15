// Package admin 实现 RAG 系统的运维侧能力：链路追踪、概览统计、运维工具。
//
// 本文件把 admin 路由挂到上层 RouterGroup。调用方负责挂鉴权中间件；MVP 阶段
// 只用 JWT 鉴权，role 校验（仅 admin 角色可访问）留待后续——届时在这里加一个
// role middleware 即可，路由结构不变。
package admin

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册 admin 模块路由。
//
// 当前只有 trace 列表 + 详情；后续 dashboard 概览接口也挂在 /admin 下。
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	traces := rg.Group("/admin/traces")
	traces.GET("", h.ListTraces)
	traces.GET("/:id", h.GetTrace)
}
