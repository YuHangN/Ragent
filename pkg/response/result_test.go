package response

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuccess_NoData(t *testing.T) {
	r := Success[any](nil)
	assert.Equal(t, "0", r.Code)
	assert.Nil(t, r.Data)
	assert.Empty(t, r.Message)
}

func TestSuccess_WithData(t *testing.T) {
	type Payload struct{ Name string }
	r := Success(Payload{Name: "ragent"})
	assert.Equal(t, "0", r.Code)
	assert.Equal(t, "ragent", r.Data.Name)
}

func TestFail(t *testing.T) {
	r := Fail[any]("A000001", "服务异常")
	assert.Equal(t, "A000001", r.Code)
	assert.Equal(t, "服务异常", r.Message)
	assert.Nil(t, r.Data)
}
