package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.Use(ErrorHandler())
	return r
}

func TestErrorHandler_ClientError(t *testing.T) {
	r := setupEngine()
	r.GET("/client", func(c *gin.Context) {
		_ = c.Error(apperror.NewClient(errorcode.UserNameExistError))
	})

	req := httptest.NewRequest(http.MethodGet, "/client", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var res response.Result[any]
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, "A000111", res.Code)
	assert.Equal(t, "用户名已存在", res.Message)
}

func TestErrorHandler_ServiceError(t *testing.T) {
	r := setupEngine()
	r.GET("/service", func(c *gin.Context) {
		_ = c.Error(apperror.NewServiceMsg("业务出错"))
	})

	req := httptest.NewRequest(http.MethodGet, "/service", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var res response.Result[any]
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, "B000001", res.Code)
}

func TestErrorHandler_RemoteError(t *testing.T) {
	r := setupEngine()
	r.GET("/remote", func(c *gin.Context) {
		_ = c.Error(apperror.NewRemote("下游服务超时"))
	})

	req := httptest.NewRequest(http.MethodGet, "/remote", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var res response.Result[any]
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, "C000001", res.Code)
}

func TestRecovery_PanicWithAppError(t *testing.T) {
	r := setupEngine()
	r.GET("/panic-app", func(c *gin.Context) {
		panic(apperror.NewClient(errorcode.UserNameExistError))
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-app", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var res response.Result[any]
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, "A000111", res.Code)
}

func TestRecovery_PanicWithString(t *testing.T) {
	r := setupEngine()
	r.GET("/panic-str", func(c *gin.Context) {
		panic("something broke")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-str", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var res response.Result[any]
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, "B000001", res.Code)
}

func TestHandleError_Helper(t *testing.T) {
	r := setupEngine()
	r.GET("/helper", func(c *gin.Context) {
		HandleError(c, apperror.NewServiceMsg("手动处理"))
	})

	req := httptest.NewRequest(http.MethodGet, "/helper", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
