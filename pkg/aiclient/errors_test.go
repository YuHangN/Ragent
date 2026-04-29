package aiclient

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyHTTP(t *testing.T) {
	cases := []struct {
		status int
		want   ErrorType
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrUnauthorized},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusBadRequest, ErrClientError},
		{http.StatusNotFound, ErrClientError},
		{http.StatusInternalServerError, ErrServerError},
		{http.StatusBadGateway, ErrServerError},
		{http.StatusGatewayTimeout, ErrServerError},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, ClassifyHTTP(c.status), "status=%d", c.status)
	}
}

func TestClientError_Error(t *testing.T) {
	e := &ClientError{
		Type:       ErrRateLimited,
		StatusCode: 429,
		Message:    "rate limit exceeded",
	}
	assert.Contains(t, e.Error(), "RATE_LIMITED")
	assert.Contains(t, e.Error(), "429")
	assert.Contains(t, e.Error(), "rate limit exceeded")
}

func TestClientError_AsCheck(t *testing.T) {
	e := &ClientError{Type: ErrServerError, StatusCode: 500}
	var ce *ClientError
	assert.True(t, errors.As(e, &ce))
	assert.Equal(t, ErrServerError, ce.Type)
}

func TestErrorType_IsRetryable(t *testing.T) {
	assert.True(t, ErrRateLimited.IsRetryable())
	assert.True(t, ErrServerError.IsRetryable())
	assert.True(t, ErrNetworkError.IsRetryable())
	assert.False(t, ErrUnauthorized.IsRetryable()) // 401 重试无意义
	assert.False(t, ErrClientError.IsRetryable())  // 4xx 重试无意义
}
