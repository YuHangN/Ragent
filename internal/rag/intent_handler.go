// Package rag 包含检索增强生成（RAG）相关的核心业务逻辑。
//
// 本文件提供意图树管理的 HTTP 入口，以及 Phase 6 阶段的 RAG 调试端点。意图树是
// RAG 路由的"骨架"：每个知识库下都可以维护一棵意图节点树，分类器和检索通道根据
// 它判断用户问题该走哪条路径。前端通过这些接口创建 / 删除节点、读取整棵树；
// /test-retrieve 则在 Phase 7 chat 上线前提供独立的检索回归手段，方便人工核对
// 改写、意图、召回质量。
package rag

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// IntentHandler 处理意图树 CRUD 与调试检索请求。
// 对应 Java：IntentTreeController + 调试端点。
//
// repo 提供持久化访问；ragCore 仅用于 /test-retrieve。允许 ragCore 为 nil，
// 这样在尚未启用 RAG 主链路（例如某些测试场景）时仍可挂载意图树管理接口，调试
// 端点会显式返回 503 而不是 panic。
type IntentHandler struct {
	repo    IntentRepo
	ragCore *RAGCoreService
}

// NewIntentHandler 构造意图树 Handler。ragCore 可传 nil，仅影响 /test-retrieve。
func NewIntentHandler(repo IntentRepo, ragCore *RAGCoreService) *IntentHandler {
	return &IntentHandler{repo: repo, ragCore: ragCore}
}

// intentNodeCreateRequest 是创建意图节点的入参。
//
// 字段命名与意图节点表保持一致，方便前端直接传 JSON。Kind 必填；CollectionName
// 仅在 Kind=KB 时生效，MCPToolID 仅在 Kind=MCP 时生效（Phase 10）。
type intentNodeCreateRequest struct {
	KbID           int64      `json:"kbId" binding:"required"`
	ParentID       *int64     `json:"parentId"`
	Name           string     `json:"name" binding:"required"`
	Description    string     `json:"description"`
	Examples       string     `json:"examples"`
	Level          int        `json:"level" binding:"required"`
	Kind           IntentKind `json:"kind" binding:"required"`
	CollectionName string     `json:"collectionName"`
	MCPToolID      string     `json:"mcpToolId"`
	PromptSnippet  string     `json:"promptSnippet"`
	TopK           *int       `json:"topK"`
	SortOrder      int        `json:"sortOrder"`
}

// CreateIntentNode 创建一个意图节点。POST /intent-nodes
//
// 创建成功后返回新节点的字符串 ID。新节点默认启用（Enabled=1）。
func (h *IntentHandler) CreateIntentNode(c *gin.Context) {
	var req intentNodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}

	node := &IntentNode{
		KbID:           req.KbID,
		ParentID:       req.ParentID,
		Name:           req.Name,
		Description:    req.Description,
		Examples:       req.Examples,
		Level:          req.Level,
		Kind:           req.Kind,
		CollectionName: req.CollectionName,
		MCPToolID:      req.MCPToolID,
		PromptSnippet:  req.PromptSnippet,
		TopK:           req.TopK,
		SortOrder:      req.SortOrder,
		Enabled:        1,
	}
	if err := h.repo.Create(node); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"id": strconv.FormatInt(node.ID, 10)}))
}

// GetIntentTree 返回某个知识库下的完整意图树。GET /intent-nodes?kbId=xxx
//
// 数据库里的节点是扁平存储的；这里通过 BuildTree 重建嵌套结构，便于前端做菜单
// 渲染或拖拽编辑。
func (h *IntentHandler) GetIntentTree(c *gin.Context) {
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

// DeleteIntentNode 删除单个意图节点。DELETE /intent-nodes/:id
//
// 软删除由 GORM 配置的 deleted 字段处理；这里只负责入参解析和错误透传。
func (h *IntentHandler) DeleteIntentNode(c *gin.Context) {
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

// testRetrieveRequest 是 /test-retrieve 的入参。
//
// KbIDs 在前端传字符串数组，避免大整数 ID 在 JS 里精度丢失；服务端按字符串解析
// int64，无法解析的条目会被忽略而不是报错，使得调试时即使传入个别脏数据也不会
// 整体失败。
type testRetrieveRequest struct {
	KbIDs    []string `json:"kbIds"`
	Question string   `json:"question" binding:"required"`
	TopK     int      `json:"topK"`
}

// TestRetrieve 是 Phase 6 的调试端点。POST /api/rag/test-retrieve
//
// 它直接调用 RAGCoreService.Retrieve，跳过 Phase 7 的 chat 包装，让我们能在
// chat 上线前独立验证：① 改写质量；② 意图分类与短路是否正确；③ 多通道召回与
// 重排顺序。Phase 7 上线后该接口仍可保留作为 ops 工具。
func (h *IntentHandler) TestRetrieve(c *gin.Context) {
	if h.ragCore == nil {
		_ = c.Error(apperror.NewClientMsg("RAG 核心服务未启用"))
		return
	}
	var req testRetrieveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	kbIDs := make([]int64, 0, len(req.KbIDs))
	for _, s := range req.KbIDs {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			kbIDs = append(kbIDs, id)
		}
	}
	result, err := h.ragCore.Retrieve(c.Request.Context(), RetrieveRequest{
		KbIDs:    kbIDs,
		Question: req.Question,
		TopK:     req.TopK,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}
