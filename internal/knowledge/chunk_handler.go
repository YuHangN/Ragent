package knowledge

import (
	"net/http"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
)

type ChunkHandler struct {
	svc *ChunkService
}

func NewChunkHandler(svc *ChunkService) *ChunkHandler {
	return &ChunkHandler{svc: svc}
}

// PageChunks GET /knowledge-base/docs/:doc-id/chunks
func (h *ChunkHandler) PageChunks(c *gin.Context) {
	docID := c.Param("doc-id")
	var req ChunkPageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	result, err := h.svc.Page(docID, req.Enabled, req.Current, req.Size)
	if err != nil {
		_ = c.Error(apperror.NewServiceWrap("查询失败", err, errorcode.ServiceError))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// CreateChunk POST /knowledge-base/docs/:doc-id/chunks
func (h *ChunkHandler) CreateChunk(c *gin.Context) {
	var req ChunkCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	vo, err := h.svc.Create(c.Param("doc-id"), req.Content, usercontext.Username(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

// UpdateChunk PUT /knowledge-base/docs/:doc-id/chunks/:chunk-id
func (h *ChunkHandler) UpdateChunk(c *gin.Context) {
	var req ChunkUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.svc.Update(c.Param("doc-id"), c.Param("chunk-id"), req.Content, usercontext.Username(c)); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// DeleteChunk DELETE /knowledge-base/docs/:doc-id/chunks/:chunk-id
func (h *ChunkHandler) DeleteChunk(c *gin.Context) {
	if err := h.svc.Delete(c.Param("doc-id"), c.Param("chunk-id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// EnableChunk POST /knowledge-base/docs/:doc-id/chunks/:chunk-id/enable|disable
func (h *ChunkHandler) EnableChunk(c *gin.Context) {
	enabled := !strings.HasSuffix(c.Request.URL.Path, "disable")
	if err := h.svc.EnableChunk(c.Param("doc-id"), c.Param("chunk-id"), enabled, usercontext.Username(c)); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// BatchEnableChunks POST /knowledge-base/docs/:doc-id/chunks/batch-enable|batch-disable
func (h *ChunkHandler) BatchEnableChunks(c *gin.Context) {
	enabled := !strings.HasSuffix(c.Request.URL.Path, "disable")
	var req ChunkBatchRequest
	_ = c.ShouldBindJSON(&req) // batch body 允许为空
	if err := h.svc.BatchEnable(c.Param("doc-id"), req.IDs, enabled, usercontext.Username(c)); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}
