// Package admin 实现 RAG 系统的运维侧能力：链路追踪、概览统计、运维工具。
//
// 本文件提供 trace 的 HTTP 入口：列表 + 详情。MVP 不做筛选 / 聚合 / 趋势图，
// 那些留待后续 dashboard 阶段。
package admin

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler 是 admin 模块的 HTTP 入口。
//
// MVP 只持有 TraceRepo——纯读路径，没有业务规则，不需要 service 层。后续加
// dashboard 聚合统计时再引入 DashboardService。
type Handler struct {
	repo TraceRepo
}

// NewHandler 构造 admin Handler。
func NewHandler(repo TraceRepo) *Handler {
	return &Handler{repo: repo}
}

// ListTraces 返回 trace 列表。GET /admin/traces?limit=&offset=
//
// 按 create_time 倒序，最近的在前。limit / offset 缺省时由 Repo 兜底。
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

// GetTrace 返回单条 trace 详情。GET /admin/traces/:id
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
