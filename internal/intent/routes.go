package intent

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	nodes := rg.Group("/intent-nodes")
	nodes.POST("", h.CreateIntentNode)
	nodes.GET("", h.GetIntentTree)
	nodes.DELETE("/:id", h.DeleteIntentNode)
}
