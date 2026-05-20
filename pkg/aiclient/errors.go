package aiclient

import (
	"fmt"
	"net/http"
)

// ErrorType 表示模型调用失败的分类。
type ErrorType string

const (
	// ErrUnauthorized 表示凭证缺失、无效或无权限。
	ErrUnauthorized ErrorType = "UNAUTHORIZED"
	// ErrRateLimited 表示请求被限流或触发配额限制。
	ErrRateLimited ErrorType = "RATE_LIMITED"
	// ErrServerError 表示 provider 返回 5xx 服务端错误。
	ErrServerError ErrorType = "SERVER_ERROR"
	// ErrClientError 表示非鉴权类的 4xx 客户端错误。
	ErrClientError ErrorType = "CLIENT_ERROR"
	// ErrNetworkError 表示请求未能正常到达 provider 或读取响应。
	ErrNetworkError ErrorType = "NETWORK_ERROR"
	// ErrInvalidResponse 表示 provider 响应格式无法被当前客户端使用。
	ErrInvalidResponse ErrorType = "INVALID_RESPONSE"
	// ErrProviderError 表示无法归入其他类型的 provider 错误。
	ErrProviderError ErrorType = "PROVIDER_ERROR"
)

// IsRetryable 判断同一个请求是否值得重试。
func (t ErrorType) IsRetryable() bool {
	switch t {
	case ErrRateLimited, ErrServerError, ErrNetworkError, ErrProviderError:
		return true
	default:
		return false
	}
}

// ClientError 表示一次模型客户端调用中的结构化错误。
type ClientError struct {
	Type       ErrorType
	StatusCode int
	Message    string
	Cause      error
}

// Error 返回包含错误类型、HTTP 状态码和消息的可读错误文本。
func (e *ClientError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%s] HTTP %d: %s", e.Type, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap 返回底层错误，便于 errors.Is / errors.As 继续匹配。
func (e *ClientError) Unwrap() error { return e.Cause }

// ClassifyHTTP 将 HTTP 状态码映射为包内统一的错误类型。
func ClassifyHTTP(status int) ErrorType {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ErrUnauthorized
	case status == http.StatusTooManyRequests:
		return ErrRateLimited
	case status >= 500 && status < 600:
		return ErrServerError
	case status >= 400 && status < 500:
		return ErrClientError
	default:
		return ErrProviderError
	}
}

// NewHTTPError 根据非 200 HTTP 响应构造 ClientError。
func NewHTTPError(status int, body string) *ClientError {
	return &ClientError{
		Type:       ClassifyHTTP(status),
		StatusCode: status,
		Message:    body,
	}
}
