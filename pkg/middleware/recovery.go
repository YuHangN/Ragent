package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 捕获 handler 中的 panic，记录日志并返回统一 500 错误响应。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				if c.Writer.Written() {
					return
				}
				// panic 值是 AppError（或包装）→ 走统一分类响应
				if err, ok := r.(error); ok {
					var appErr *apperror.AppError
					if errors.As(err, &appErr) {
						writeErrorResponse(c, appErr)
						return
					}
					zap.L().Error("panic recovered",
						zap.Error(err),
						zap.String("path", c.Request.URL.Path),
					)
				} else {
					zap.L().Error("panic recovered",
						zap.Any("value", r),
						zap.String("path", c.Request.URL.Path),
					)
				}
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					response.Fail[any](errorcode.ServiceError.Code(), fmt.Sprintf("服务器内部错误: %v", r)),
				)
			}
		}()
		c.Next()
	}
}
