package retrieval

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// TestRetrieveHandler 提供 /test-retrieve 调试接口，手动触发一次 RAG 检索
// 查看改写 / 意图分类 / 召回 / 排序的中间结果。
type TestRetrieveHandler struct {
	ragCore *RAGCoreService
}

func NewTestRetrieveHandler(ragCore *RAGCoreService) *TestRetrieveHandler {
	return &TestRetrieveHandler{ragCore: ragCore}
}

type testRetrieveRequest struct {
	KbIDs    []string `json:"kbIds"`
	Question string   `json:"question" binding:"required"`
	TopK     int      `json:"topK"`
}

func (h *TestRetrieveHandler) Handle(c *gin.Context) {
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
