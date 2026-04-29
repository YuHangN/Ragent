package aiclient

import (
	"fmt"
	"net/http"
)

// ErrorType 模型客户端错误分类
type ErrorType string

const (
	ErrUnauthorized    ErrorType = "UNAUTHORIZED"
	ErrRateLimited     ErrorType = "RATE_LIMITED"
	ErrServerError     ErrorType = "SERVER_ERROR"
	ErrClientError     ErrorType = "CLIENT_ERROR"
	ErrNetworkError    ErrorType = "NETWORK_ERROR"
	ErrInvalidResponse ErrorType = "INVALID_RESPONSE"
	ErrProviderError   ErrorType = "PROVIDER_ERROR"
)

// IsRetryable 该错误是否值得重试。
func (t ErrorType) IsRetryable() bool {
	switch t {
	case ErrRateLimited, ErrServerError, ErrNetworkError, ErrProviderError:
		return true
	default:
		return false
	}
}

// ClientError 模型客户端调用错误
type ClientError struct {
	Type       ErrorType
	StatusCode int
	Message    string
	Cause      error
}

func (e *ClientError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%s] HTTP %d: %s", e.Type, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func (e *ClientError) Unwrap() error { return e.Cause }

// ClassifyHTTP 把 HTTP 状态码映射到 ErrorType
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

// NewHTTPError 是各 client 拿到非 200 响应时的便捷构造。
func NewHTTPError(status int, body string) *ClientError {
	return &ClientError{
		Type:       ClassifyHTTP(status),
		StatusCode: status,
		Message:    body,
	}
}
