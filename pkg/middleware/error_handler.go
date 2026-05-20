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

// ErrorHandler 将 gin.Context 中收集到的错误转换为统一 HTTP 响应。
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
//
// 它会立即中断请求并写出统一错误响应。
// 适合在出错后想立刻中断的场景。
func HandleError(c *gin.Context, err error) {
	c.Abort()
	writeErrorResponse(c, err)
}

// writeErrorResponse 是错误响应写入的单点实现。
func writeErrorResponse(c *gin.Context, err error) {
	// 已写过响应则跳过，避免重复写入。
	if c.Writer.Written() {
		return
	}

	// 参数校验错误返回 400，并尽量给出首个字段的校验提示。
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

	// AppError 按错误类别映射 HTTP 状态码，同时保留业务错误码。
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

	// 未识别错误统一隐藏细节，避免把内部异常暴露给调用方。
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
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}

// firstValidationMessage 返回参数校验错误中的首个字段提示。
func firstValidationMessage(errs validator.ValidationErrors) string {
	if len(errs) == 0 {
		return errorcode.ClientError.Message()
	}
	fe := errs[0]
	// 后续可按 tag 扩展更友好的提示，例如 required、email、min。
	return fe.Field() + " " + fe.Tag()
}
