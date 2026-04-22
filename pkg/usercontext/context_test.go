package usercontext

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newCtx() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c
}

func TestSetAndGet(t *testing.T) {
	c := newCtx()
	u := &LoginUser{UserID: "1", Username: "alice", Role: "admin", Avatar: "a.png"}
	Set(c, u)

	got, ok := Get(c)
	assert.True(t, ok)
	assert.Equal(t, "1", got.UserID)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, "admin", got.Role)
	assert.Equal(t, "a.png", got.Avatar)
}

func TestGet_NotSet(t *testing.T) {
	c := newCtx()
	_, ok := Get(c)
	assert.False(t, ok)
}

func TestRequire_Success(t *testing.T) {
	c := newCtx()
	Set(c, &LoginUser{UserID: "1", Username: "bob", Role: "user"})

	u := Require(c)
	assert.Equal(t, "bob", u.Username)
}

func TestRequire_PanicsWhenMissing(t *testing.T) {
	c := newCtx()
	assert.Panics(t, func() {
		Require(c)
	})
}

func TestUserID(t *testing.T) {
	c := newCtx()
	assert.Equal(t, "", UserID(c))

	Set(c, &LoginUser{UserID: "42"})
	assert.Equal(t, "42", UserID(c))
}
