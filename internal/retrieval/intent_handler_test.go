package retrieval

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRouter 构造一个挂了 error handler middleware 的 gin engine，
// 用来覆盖 handler 里 `c.Error(apperror.NewClientMsg(...))` 之后的 HTTP 返回。
func newTestRouter(h *IntentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/intent-nodes", h.CreateIntentNode)
	return r
}

// TestCreateIntentNode_KbKindRequiresPartitionName 验证 Phase 6.7 新增的校验：
// Kind=KB 的意图节点不带 PartitionName 时，handler 拒绝创建。
func TestCreateIntentNode_KbKindRequiresPartitionName(t *testing.T) {
	repo := &stubIntentRepo{}
	h := NewIntentHandler(repo, nil)
	router := newTestRouter(h)

	body := map[string]any{
		"kbId":  100,
		"name":  "无 partition 的 KB 节点",
		"level": 1,
		"kind":  "KB",
		// partitionName 故意留空
	}
	buf, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/intent-nodes", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 期望 4xx，并且响应体里能看到校验文案
	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "partitionName")
}

// TestCreateIntentNode_SystemKindNoPartitionOK 验证 SYSTEM 节点不强制 PartitionName。
func TestCreateIntentNode_SystemKindNoPartitionOK(t *testing.T) {
	repo := &stubIntentRepo{}
	h := NewIntentHandler(repo, nil)
	router := newTestRouter(h)

	body := map[string]any{
		"kbId":  100,
		"name":  "闲聊问候",
		"level": 1,
		"kind":  "SYSTEM",
	}
	buf, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/intent-nodes", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "SYSTEM 节点不应被 partition 校验拦截，响应: %s", w.Body.String())
}
