package apperror

import (
	"errors"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/stretchr/testify/assert"
)

func TestClientError_WithErrorCode(t *testing.T) {
	err := NewClient(errorcode.UserNameExistError)
	assert.Equal(t, "A000111", err.Code())
	assert.Equal(t, "用户名已存在", err.Message())
	assert.Equal(t, KindClient, err.Kind())
}

func TestClientError_WithCustomMessage(t *testing.T) {
	err := NewClientMsg("自定义提示")
	assert.Equal(t, "A000001", err.Code())
	assert.Equal(t, "自定义提示", err.Message())
}

func TestServiceError_Default(t *testing.T) {
	err := NewServiceMsg("业务出错")
	assert.Equal(t, "B000001", err.Code())
	assert.Equal(t, KindService, err.Kind())
}

func TestRemoteError_WithCause(t *testing.T) {
	cause := errors.New("network failure")
	err := NewRemoteWrap("第三方调用失败", cause, errorcode.RemoteError)
	assert.ErrorIs(t, err, cause)
	assert.Equal(t, "C000001", err.Code())
}

func TestErrorsAs_TypeAssertion(t *testing.T) {
	err := NewClient(errorcode.UserNameExistError)

	var appErr *AppError
	ok := errors.As(err, &appErr)
	assert.True(t, ok)
	assert.Equal(t, KindClient, appErr.Kind())
}

func TestDomainError_VectorCollectionAlreadyExists(t *testing.T) {
	err := NewVectorCollectionAlreadyExists("kb_123")
	assert.Equal(t, KindService, err.Kind())
	assert.Contains(t, err.Message(), "kb_123")
}
