package apperror

import (
	"github.com/YuHangN/ragent-go/pkg/errorcode"
)

func NewClient(ec errorcode.IErrorCode) *AppError {
	return newAppError(KindClient, "", nil, ec)
}

func NewClientMsg(message string) *AppError {
	return newAppError(KindClient, message, nil, errorcode.ClientError)
}

func NewClientMsgCode(message string, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindClient, message, nil, ec)
}

func NewClientWrap(message string, cause error, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindClient, message, cause, ec)
}
