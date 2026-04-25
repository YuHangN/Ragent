package knowledge

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 将 Knowledge 模块的所有路由挂载到 RouterGroup。
// 拆成 3 个 Handler 对齐 Java KnowledgeBaseController / KnowledgeDocumentController / KnowledgeChunkController。
func RegisterRoutes(
	rg *gin.RouterGroup,
	kbH *KBHandler,
	docH *DocHandler,
	chunkH *ChunkHandler,
	auth gin.HandlerFunc,
) {
	kb := rg.Group("/knowledge-base", auth)

	// 知识库 CRUD
	kb.POST("", kbH.CreateKB)
	kb.GET("", kbH.PageKB)
	kb.GET("/:kb-id", kbH.GetKB)
	kb.PUT("/:kb-id", kbH.RenameKB)
	kb.DELETE("/:kb-id", kbH.DeleteKB)

	// 文档管理（挂在 KB 下）
	kb.POST("/:kb-id/docs/upload", docH.UploadDoc)
	kb.GET("/:kb-id/docs", docH.PageDocs)

	// 文档管理（独立路径）
	doc := rg.Group("/knowledge-base/docs", auth)
	doc.GET("/search", docH.SearchDocs) // 先注册 /search 避免和 :doc-id 冲突
	doc.GET("/:doc-id", docH.GetDoc)
	doc.PUT("/:doc-id", docH.UpdateDoc)
	doc.DELETE("/:doc-id", docH.DeleteDoc)
	doc.POST("/:doc-id/chunk", docH.StartChunk)
	doc.PATCH("/:doc-id/enable", docH.EnableDoc)
	doc.GET("/:doc-id/chunk-logs", docH.GetChunkLogs)

	// Chunk 管理
	chunk := rg.Group("/knowledge-base/docs/:doc-id/chunks", auth)
	chunk.GET("", chunkH.PageChunks)
	chunk.POST("", chunkH.CreateChunk)
	chunk.PUT("/:chunk-id", chunkH.UpdateChunk)
	chunk.DELETE("/:chunk-id", chunkH.DeleteChunk)
	chunk.POST("/:chunk-id/enable", chunkH.EnableChunk)
	chunk.POST("/:chunk-id/disable", chunkH.EnableChunk)
	chunk.POST("/batch-enable", chunkH.BatchEnableChunks)
	chunk.POST("/batch-disable", chunkH.BatchEnableChunks)
}
