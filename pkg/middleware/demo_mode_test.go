package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func demoRouter(enabled bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(DemoMode(enabled))
	r.GET("/items", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.POST("/items", func(c *gin.Context) { c.String(http.StatusOK, "created") })
	r.OPTIONS("/items", func(c *gin.Context) { c.String(http.StatusOK, "preflight") })
	return r
}

func TestDemoMode_Disabled_AllowsAll(t *testing.T) {
	r := demoRouter(false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/items", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDemoMode_Enabled_AllowsGet(t *testing.T) {
	r := demoRouter(true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/items", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDemoMode_Enabled_AllowsOptions(t *testing.T) {
	r := demoRouter(true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, "/items", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "preflight", w.Body.String())
}

func TestDemoMode_Enabled_RejectsPost(t *testing.T) {
	r := demoRouter(true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/items", nil))
	assert.Equal(t, http.StatusOK, w.Code) // Java 版也是返回 200 带 code A000001
	assert.Contains(t, w.Body.String(), "体验环境仅支持查询操作")
}
