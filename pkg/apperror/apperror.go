package apperror

import (
	"fmt"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
)

// Kind 表示应用错误的来源类别。
//
// middleware 可根据 Kind 将错误映射为不同 HTTP 状态码。
type Kind int

const (
	// KindUnknown 表示未明确分类的错误。
	KindUnknown Kind = iota
	// KindClient 表示由客户端输入或请求参数导致的错误，通常映射为 HTTP 400。
	KindClient
	// KindService 表示服务内部处理失败，通常映射为 HTTP 500。
	KindService
	// KindRemote 表示远端依赖调用失败，通常映射为 HTTP 502。
	KindRemote
)

// String 返回错误类别的可读名称。
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

// AppError 是应用层统一错误结构。
//
// AppError 同时保存错误类别、错误码、展示消息和底层 cause。客户端错误、
// 服务端错误和远端依赖错误都共用该结构，并通过 kind 字段区分。
type AppError struct {
	kind    Kind
	code    string
	message string
	cause   error // 原始错误，用于 errors.Is / errors.As 链式追溯。
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

// Kind 返回错误来源类别。
func (e *AppError) Kind() Kind { return e.kind }

// Code 返回统一错误码。
func (e *AppError) Code() string { return e.code }

// Message 返回可展示的错误消息。
func (e *AppError) Message() string { return e.message }

// newAppError 是所有构造函数的公共内部实现。
//
// message 为空时使用 errorCode.Message()；errorCode 为空时使用默认服务端错误码。
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
