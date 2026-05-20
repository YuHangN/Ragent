package apperror

import (
	"github.com/YuHangN/ragent-go/pkg/errorcode"
)

// NewClient 使用指定错误码构造客户端错误。
func NewClient(ec errorcode.IErrorCode) *AppError {
	return newAppError(KindClient, "", nil, ec)
}

// NewClientMsg 使用默认客户端错误码和自定义消息构造客户端错误。
func NewClientMsg(message string) *AppError {
	return newAppError(KindClient, message, nil, errorcode.ClientError)
}

// NewClientMsgCode 使用指定错误码和自定义消息构造客户端错误。
func NewClientMsgCode(message string, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindClient, message, nil, ec)
}

// NewClientWrap 使用指定错误码、自定义消息和底层原因构造客户端错误。
func NewClientWrap(message string, cause error, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindClient, message, cause, ec)
}
