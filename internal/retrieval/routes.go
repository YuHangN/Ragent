// Package rag 包含检索增强生成（RAG）相关的核心业务逻辑。
//
// 本文件负责把意图树管理 + 调试检索的 HTTP 入口挂到路由树上。意图树管理（CRUD）
// 与调试检索都属于内部运维能力，按需要可以加鉴权中间件；当前先按 main.go 注入
// 决定是否包鉴权，路由本身保持纯粹。
package retrieval

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册 RAG 模块的全部 HTTP 路由。
//
// /intent-nodes 下挂载意图节点 CRUD；/rag/test-retrieve 是 Phase 6 调试端点，
// 直接驱动 RAGCoreService.Retrieve，便于 chat 上线前人工核对召回质量。
func RegisterRoutes(rg *gin.RouterGroup, h *IntentHandler) {
	nodes := rg.Group("/intent-nodes")
	nodes.POST("", h.CreateIntentNode)
	nodes.GET("", h.GetIntentTree)
	nodes.DELETE("/:id", h.DeleteIntentNode)

	rg.POST("/rag/test-retrieve", h.TestRetrieve)
}
