// Package retrieval 提供 RAG 检索的 HTTP 入口。
//
// 当前只暴露 /rag/test-retrieve 调试端点；意图节点 CRUD 由 intent 包自行注册。
package retrieval

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *TestRetrieveHandler) {
	rg.POST("/rag/test-retrieve", h.Handle)
}
