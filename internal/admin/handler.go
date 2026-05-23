// Package admin 提供 RAG 系统的运维接口与链路追踪能力。
//
// 本文件提供 trace 的 HTTP 入口，包括列表查询和详情查询。
package admin

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler 是 admin 模块的 HTTP 处理器。
//
// 当前接口只编排 TraceRepo 的读路径，暂不引入额外 service 层。
type Handler struct {
	repo TraceRepo
}

// NewHandler 创建 admin Handler。
func NewHandler(repo TraceRepo) *Handler {
	return &Handler{repo: repo}
}

// ListTraces 处理 GET /admin/traces?limit=&offset=，返回 trace 列表。
//
// 结果按 create_time 倒序排列，limit 和 offset 缺省值由 Repo 兜底。
func (h *Handler) ListTraces(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	list, total, err := h.repo.List(limit, offset)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{
		"total": total,
		"list":  toListItems(list),
	}))
}

// GetTrace 处理 GET /admin/traces/:id，返回单条 trace 详情。
func (h *Handler) GetTrace(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("trace ID 格式错误"))
		return
	}
	t, err := h.repo.FindByID(id)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(toDetail(*t)))
}
