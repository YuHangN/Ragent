package apperror

import "github.com/YuHangN/ragent-go/pkg/errorcode"

// NewRemote 对应 Java：new RemoteException(message)，默认 REMOTE_ERROR 码。
func NewRemote(message string) *AppError {
	return newAppError(KindRemote, message, nil, errorcode.RemoteError)
}

// NewRemoteMsgCode 对应 Java：new RemoteException(message, errorCode)。
func NewRemoteMsgCode(message string, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindRemote, message, nil, ec)
}

// NewRemoteWrap 对应 Java：new RemoteException(message, throwable, errorCode)。
func NewRemoteWrap(message string, cause error, ec errorcode.IErrorCode) *AppError {
	return newAppError(KindRemote, message, cause, ec)
}
