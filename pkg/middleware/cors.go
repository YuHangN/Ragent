package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 返回允许所有来源访问的跨域中间件。
//
// OPTIONS 预检请求会直接返回 204，不再继续执行后续 handler。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Expose-Headers", "Authorization")
		c.Header("Access-Control-Max-Age", "3600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
