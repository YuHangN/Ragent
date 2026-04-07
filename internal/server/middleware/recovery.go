package middleware

import (
	"net/http"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 捕获 handler 中的 panic，记录日志并返回统一 500 错误响应。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// zap.L() 获取全局 logger，在 main.go 中通过 zap.ReplaceGlobals 注册。
				zap.L().Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					response.Fail[any](errorcode.ServiceError, "服务器内部错误"),
				)
			}
		}()
		c.Next()
	}
}
