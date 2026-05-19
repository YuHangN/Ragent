// Package intent 提供意图节点的 HTTP 接口与意图树构建。
//
// 调试检索接口（/test-retrieve）属于 retrieval 域，放在 retrieval 包中暴露，
// 这里只负责意图节点 CRUD。
package intent

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler 处理意图节点 CRUD 接口。
type Handler struct {
	repo Repo
}

func NewHandler(repo Repo) *Handler {
	return &Handler{repo: repo}
}

// nodeCreateRequest 是创建意图节点的入参。
//
// 常见例子：
//   - Kind=KB 时，PartitionName 表示 KB collection 下要检索的 Milvus partition 名（如 "install" / "refund"）。
//   - Kind=SYSTEM 时，通常不需要 PartitionName。
//   - Kind=MCP 时，MCPToolID 表示要调用的外部工具。
type nodeCreateRequest struct {
	KbID          int64  `json:"kbId" binding:"required"`
	ParentID      *int64 `json:"parentId"`
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	Examples      string `json:"examples"`
	Level         int    `json:"level" binding:"required"`
	Kind          Kind   `json:"kind" binding:"required"`
	PartitionName string `json:"partitionName"`
	MCPToolID     string `json:"mcpToolId"`
	PromptSnippet string `json:"promptSnippet"`
	TopK          *int   `json:"topK"`
	SortOrder     int    `json:"sortOrder"`
}

// CreateIntentNode 创建一个意图节点。
//
// 路由：POST /intent-nodes
//
// 新节点默认启用，Enabled=1；ID 用字符串返回避免前端 int64 精度问题。
func (h *Handler) CreateIntentNode(c *gin.Context) {
	var req nodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}

	// Kind=KB 必须指定 partition——否则 ingestion 不知道写到哪个 partition，
	// channel 检索也无法精准缩范围。其它 Kind 不强制。
	if req.Kind == KindKB && strings.TrimSpace(req.PartitionName) == "" {
		_ = c.Error(apperror.NewClientMsg("Kind=KB 的意图节点必须指定 partitionName"))
		return
	}

	node := &Node{
		KbID:          req.KbID,
		ParentID:      req.ParentID,
		Name:          req.Name,
		Description:   req.Description,
		Examples:      req.Examples,
		Level:         req.Level,
		Kind:          req.Kind,
		PartitionName: req.PartitionName,
		MCPToolID:     req.MCPToolID,
		PromptSnippet: req.PromptSnippet,
		TopK:          req.TopK,
		SortOrder:     req.SortOrder,
		Enabled:       1,
	}
	if err := h.repo.Create(node); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"id": strconv.FormatInt(node.ID, 10)}))
}

// GetIntentTree 返回某个知识库下的完整意图树。
//
// 路由：GET /intent-nodes?kbId=xxx
func (h *Handler) GetIntentTree(c *gin.Context) {
	kbID, err := strconv.ParseInt(c.Query("kbId"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("kbId 格式错误"))
		return
	}
	nodes, err := h.repo.FindByKbID(kbID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(BuildTree(nodes)))
}

// DeleteIntentNode 删除单个意图节点。
//
// 路由：DELETE /intent-nodes/:id
func (h *Handler) DeleteIntentNode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("id 格式错误"))
		return
	}
	if err := h.repo.Delete(id); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}
