package middleware

import (
	"net/http"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

const demoRejectMessage = "体验环境仅支持查询操作"

// DemoMode 在体验环境中拦截非查询类请求。
//
// enabled 为 false 时中间件只透传请求；为 true 时仅允许 GET 和 OPTIONS。
func DemoMode(enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		method := strings.ToUpper(c.Request.Method)
		if method == http.MethodGet || method == http.MethodOptions {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusOK,
			response.Fail[any](errorcode.ClientError.Code(), demoRejectMessage))
	}
}
