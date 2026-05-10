// Package rag 包含检索增强生成（RAG）的核心逻辑。
//
// 本文件提供意图树管理接口，以及一个用于手动验证 RAG 检索效果的调试接口。
// 意图树用于描述“用户问题可能属于哪些业务场景”，后续分类器会基于这些节点做路由。
package rag

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// IntentHandler 处理意图节点接口和调试检索接口。
//
// repo 负责读写意图节点；ragCore 只用于 TestRetrieve。
// ragCore 可以为 nil，此时意图树接口仍可使用，调试检索接口会返回错误。
type IntentHandler struct {
	repo    IntentRepo
	ragCore *RAGCoreService
}

// NewIntentHandler 创建 IntentHandler。
//
// 例子：只想启用意图树管理时，可以传入 nil ragCore；只有调用 TestRetrieve
// 时才需要 RAGCoreService。
func NewIntentHandler(repo IntentRepo, ragCore *RAGCoreService) *IntentHandler {
	return &IntentHandler{repo: repo, ragCore: ragCore}
}

// intentNodeCreateRequest 是创建意图节点的入参。
//
// 常见例子：
//   - Kind=KB 时，CollectionName 表示后续要检索的 Milvus collection。
//   - Kind=SYSTEM 时，通常不需要 CollectionName。
//   - Kind=MCP 时，MCPToolID 表示要调用的外部工具。
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

// CreateIntentNode 创建一个意图节点。
//
// 路由：POST /intent-nodes
//
// 请求体会绑定到 intentNodeCreateRequest。创建成功后返回新节点 ID，ID 用字符串返回，
// 避免前端处理 int64 时出现精度问题。新节点默认启用，Enabled=1。
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

// GetIntentTree 返回某个知识库下的完整意图树。
//
// 路由：GET /intent-nodes?kbId=xxx
//
// 数据库中节点是扁平列表，包含 ParentID。这里会先查出该 KB 的所有节点，
// 再通过 BuildTree 组装成父子结构，方便前端直接渲染树。
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

// DeleteIntentNode 删除单个意图节点。
//
// 路由：DELETE /intent-nodes/:id
//
// id 必须是 int64 格式。实际删除方式由 repo 决定；当前 GORM 模型使用 deleted 字段
// 做软删除。
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
// KbIDs 使用字符串数组，避免前端处理大整数 ID 时丢失精度。
// 服务端会逐个解析成 int64；无法解析的条目会被忽略。
type testRetrieveRequest struct {
	KbIDs    []string `json:"kbIds"`
	Question string   `json:"question" binding:"required"`
	TopK     int      `json:"topK"`
}

// TestRetrieve 手动触发一次 RAG 检索。
//
// 路由：POST /api/rag/test-retrieve
//
// 这个接口直接调用 RAGCoreService.Retrieve，适合排查某个问题的改写、意图分类、
// 召回和排序结果。
//
// 例子：请求 {"kbIds":["101","202"],"question":"产品 A 怎么安装？","topK":5}
// 会在 KB 101 和 KB 202 中执行一次检索，并返回 RetrieveResult。
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
