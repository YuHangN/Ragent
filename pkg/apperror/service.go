package apperror

import (
	"github.com/YuHangN/ragent-go/pkg/errorcode"
)

// NewService 使用指定错误码构造服务端错误。
func NewService(ec errorcode.IErrorCode) *AppError {
	return newAppError(KindService, "", nil, ec)
}

// NewServiceMsg 使用默认服务端错误码和自定义消息构造服务端错误。
func NewServiceMsg(message string) *AppError {
	return newAppError(KindService, message, nil, errorcode.ServiceError)
}

// NewServiceMsgCode 使用指定错误码和自定义消息构造服务端错误。
func NewServiceMsgCode(message string, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindService, message, nil, ec)
}

// NewServiceWrap 使用指定错误码、自定义消息和底层原因构造服务端错误。
func NewServiceWrap(message string, cause error, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindService, message, cause, ec)
}
