package middleware

import (
	"errors"
	"net/http"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors[0].Err
		writeErrorResponse(c, err)
	}
}

// HandleError 是给 handler 显式调用的便捷函数。
// 等价于：c.Error(err); c.Abort()；立即写响应。
// 适合在出错后想立刻中断的场景。
func HandleError(c *gin.Context, err error) {
	c.Abort()
	writeErrorResponse(c, err)
}

// writeErrorResponse 是响应写入的单点实现，供 Recovery / ErrorHandler / HandleError 共用
func writeErrorResponse(c *gin.Context, err error) {
	// 已写过响应则跳过（避免重复写）
	if c.Writer.Written() {
		return
	}

	// 1. 参数校验错误
	var valErrs validator.ValidationErrors
	if errors.As(err, &valErrs) {
		msg := firstValidationMessage(valErrs)
		zap.L().Warn("validation error",
			zap.String("path", c.Request.URL.Path),
			zap.String("detail", err.Error()),
		)
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError.Code(), msg))
		return
	}

	// 2. AppError
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		status := httpStatusFor(appErr.Kind())
		zap.L().Warn("app error",
			zap.String("kind", appErr.Kind().String()),
			zap.String("code", appErr.Code()),
			zap.String("message", appErr.Message()),
			zap.String("path", c.Request.URL.Path),
			zap.Error(appErr.Unwrap()),
		)
		c.JSON(status, response.Fail[any](appErr.Code(), appErr.Message()))
		return
	}

	// 3. 兜底
	zap.L().Error("unhandled error",
		zap.String("path", c.Request.URL.Path),
		zap.Error(err),
	)
	c.JSON(http.StatusInternalServerError,
		response.Fail[any](errorcode.ServiceError.Code(), "服务器内部错误"))
}

// httpStatusFor 把 AppError 类别映射成 HTTP 状态码。
func httpStatusFor(kind apperror.Kind) int {
	switch kind {
	case apperror.KindClient:
		return http.StatusBadRequest
	case apperror.KindRemote:
		return http.StatusBadGateway
	case apperror.KindService:
		// fallthrough 代表继续往下执行下一个 case 的代码块，直到遇到 break 或 switch 结束。
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}

func firstValidationMessage(errs validator.ValidationErrors) string {
	if len(errs) == 0 {
		return errorcode.ClientError.Message()
	}
	fe := errs[0]
	// 可按需扩展：根据 tag 定制消息（如 required / email / min）
	return fe.Field() + " " + fe.Tag()
}
