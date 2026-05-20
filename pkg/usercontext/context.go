package usercontext

import (
	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/gin-gonic/gin"
)

const contextKey = "ragent.loginUser"

// LoginUser 是当前请求中登录用户的上下文快照。
type LoginUser struct {
	UserID   string
	Username string
	Role     string
	Avatar   string
}

// Set 将 LoginUser 写入 gin.Context。
//
// 该函数通常由 Auth 中间件在 JWT 校验成功后调用。
func Set(c *gin.Context, u *LoginUser) {
	c.Set(contextKey, u)
}

// Get 读取当前请求的 LoginUser。
//
// 未设置或类型不匹配时返回 (nil, false)。
func Get(c *gin.Context) (*LoginUser, bool) {
	v, ok := c.Get(contextKey)
	if !ok {
		return nil, false
	}

	u, ok := v.(*LoginUser)
	if !ok {
		return nil, false
	}
	return u, true
}

// Require 获取当前登录用户。
//
// 用户不存在时会 panic 客户端错误，交由 Recovery 中间件转换成统一响应。
func Require(c *gin.Context) *LoginUser {
	u, ok := Get(c)
	if !ok {
		panic(apperror.NewClientMsg("未获取到当前登录用户"))
	}
	return u
}

// UserID 获取当前用户 ID，未登录时返回空串。
func UserID(c *gin.Context) string {
	if u, ok := Get(c); ok {
		return u.UserID
	}
	return ""
}

// Username 获取当前用户名，未登录时返回空串。
func Username(c *gin.Context) string {
	if u, ok := Get(c); ok {
		return u.Username
	}
	return ""
}

// Role 获取当前用户角色，未登录时返回空串。
func Role(c *gin.Context) string {
	if u, ok := Get(c); ok {
		return u.Role
	}
	return ""
}
