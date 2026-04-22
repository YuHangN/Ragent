package apperror

import (
	"fmt"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
)

// 用于 middleware 根据类别映射 HTTP 状态码。
type Kind int

const (
	KindUnknown Kind = iota
	KindClient       // 对应 Java ClientException → HTTP 400
	KindService      // 对应 Java ServiceException → HTTP 500
	KindRemote       // 对应 Java RemoteException → HTTP 502
)

func (k Kind) String() string {
	switch k {
	case KindClient:
		return "Client"
	case KindService:
		return "Service"
	case KindRemote:
		return "Remote"
	default:
		return "Unknown"
	}
}

// AppError 封装 errorCode + errorMessage，支持 Unwrap 保留原始 cause。
// 所有三类具体错误（ClientError / ServiceError / RemoteError）都共用此结构体，
// 通过 kind 字段区分。
type AppError struct {
	kind    Kind
	code    string
	message string
	cause   error // 原始错误，用于 errors.Is / errors.As 链式追溯
}

// Error 实现 error 接口。
func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%sError{code=%s, message=%s}: %v", e.kind, e.code, e.message, e.cause)
	}
	return fmt.Sprintf("%sError{code=%s, message=%s}", e.kind, e.code, e.message)
}

// Unwrap 支持 errors.Is / errors.As 追溯 cause。
func (e *AppError) Unwrap() error { return e.cause }

// Kind / Code / Message 提供只读访问
func (e *AppError) Kind() Kind      { return e.kind }
func (e *AppError) Code() string    { return e.code }
func (e *AppError) Message() string { return e.message }

// newAppError 是所有构造函数的公共内部实现。
// message 为空时降级取 errorCode.Message()
func newAppError(kind Kind, message string, cause error, ec errorcode.IErrorCode) *AppError {
	if ec == nil {
		ec = errorcode.ServiceError
	}
	msg := message
	if msg == "" {
		msg = ec.Message()
	}

	return &AppError{
		kind:    kind,
		code:    ec.Code(),
		message: msg,
		cause:   cause,
	}
}
