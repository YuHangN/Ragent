package knowledge

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, auth gin.HandlerFunc) {
	kb := rg.Group("/knowledge-base", auth)

	// 知识库 CRUD
	kb.POST("", h.CreateKB)
	kb.GET("", h.PageKB)
	kb.GET("/:kb-id", h.GetKB)
	kb.PUT("/:kb-id", h.RenameKB)
	kb.DELETE("/:kb-id", h.DeleteKB)

	// 文档管理（挂在 KB 下）
	kb.POST("/:kb-id/docs/upload", h.UploadDoc)
	kb.GET("/:kb-id/docs", h.PageDocs)

	// 文档管理（独立路径）
	doc := rg.Group("/knowledge-base/docs", auth)
	doc.GET("/:doc-id", h.GetDoc)
	doc.DELETE("/:doc-id", h.DeleteDoc)
	doc.POST("/:doc-id/chunk", h.StartChunk)
	doc.PATCH("/:doc-id/enable", h.EnableDoc)

	// Chunk 管理
	chunk := rg.Group("/knowledge-base/docs/:doc-id/chunks", auth)
	chunk.GET("", h.PageChunks)
	chunk.POST("", h.CreateChunk)
	chunk.PUT("/:chunk-id", h.UpdateChunk)
	chunk.DELETE("/:chunk-id", h.DeleteChunk)
	chunk.POST("/:chunk-id/enable", h.EnableChunk)
	chunk.POST("/:chunk-id/disable", h.EnableChunk)
	chunk.POST("/batch-enable", h.BatchEnableChunks)
	chunk.POST("/batch-disable", h.BatchEnableChunks)
}
