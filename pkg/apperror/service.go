package apperror

import (
	"github.com/YuHangN/ragent-go/pkg/errorcode"
)

// NewService 对应 Java：new ServiceException(errorCode)。
func NewService(ec errorcode.IErrorCode) *AppError {
	return newAppError(KindService, "", nil, ec)
}

// NewServiceMsg 对应 Java：new ServiceException(message)，默认 SERVICE_ERROR 码。
func NewServiceMsg(message string) *AppError {
	return newAppError(KindService, message, nil, errorcode.ServiceError)
}

// NewServiceMsgCode 对应 Java：new ServiceException(message, errorCode)。
func NewServiceMsgCode(message string, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindService, message, nil, ec)
}

// NewServiceWrap 对应 Java：new ServiceException(message, throwable, errorCode)。
func NewServiceWrap(message string, cause error, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindService, message, cause, ec)
}
