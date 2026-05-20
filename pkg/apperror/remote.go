package apperror

import "github.com/YuHangN/ragent-go/pkg/errorcode"

// NewRemote 使用默认远端错误码和自定义消息构造远端依赖错误。
func NewRemote(message string) *AppError {
	return newAppError(KindRemote, message, nil, errorcode.RemoteError)
}

// NewRemoteMsgCode 使用指定错误码和自定义消息构造远端依赖错误。
func NewRemoteMsgCode(message string, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindRemote, message, nil, ec)
}

// NewRemoteWrap 使用指定错误码、自定义消息和底层原因构造远端依赖错误。
func NewRemoteWrap(message string, cause error, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindRemote, message, cause, ec)
}
