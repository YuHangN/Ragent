package knowledge

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
)

type KBHandler struct {
	svc *KBService
}

func NewKBHandler(svc *KBService) *KBHandler {
	return &KBHandler{svc: svc}
}

// CreateKB POST /knowledge-base
func (h *KBHandler) CreateKB(c *gin.Context) {
	var req KBCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	id, err := h.svc.Create(req.Name, req.EmbeddingModel, usercontext.Username(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(id))
}

// RenameKB PUT /knowledge-base/:kb-id
func (h *KBHandler) RenameKB(c *gin.Context) {
	kbID, err := strconv.ParseInt(c.Param("kb-id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("知识库ID非法"))
		return
	}
	var req KBUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.svc.Rename(kbID, req.Name, usercontext.Username(c)); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// DeleteKB DELETE /knowledge-base/:kb-id
func (h *KBHandler) DeleteKB(c *gin.Context) {
	kbID, err := strconv.ParseInt(c.Param("kb-id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("知识库ID非法"))
		return
	}
	if err := h.svc.Delete(kbID); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// GetKB GET /knowledge-base/:kb-id
func (h *KBHandler) GetKB(c *gin.Context) {
	kbID, err := strconv.ParseInt(c.Param("kb-id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("知识库ID非法"))
		return
	}
	vo, err := h.svc.GetByID(kbID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

// PageKB GET /knowledge-base
func (h *KBHandler) PageKB(c *gin.Context) {
	var req KBPageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	result, err := h.svc.Page(req.Name, req.Current, req.Size)
	if err != nil {
		_ = c.Error(apperror.NewServiceWrap("查询失败", err, errorcode.ServiceError))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}
