package middleware

import (
	"net/http"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
	jwtpkg "github.com/YuHangN/ragent-go/pkg/jwt"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// Auth 验证 Authorization: Bearer <token> 头，解析成功后将用户信息写入 context。
// 下游 handler 通过 c.GetString("userID")、c.GetString("username")、c.GetString("role") 获取。
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				response.Fail[any](errorcode.Unauthorized, "未登录或登录已过期"))
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwtpkg.Parse(tokenStr, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				response.Fail[any](errorcode.Unauthorized, "未登录或登录已过期"))
			return
		}
		// 写入 context，供下游 handler 使用
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireRole 检查当前用户是否有指定角色，用于 admin 路由保护。
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := c.GetString("role")
		if current != role {
			c.AbortWithStatusJSON(http.StatusForbidden,
				response.Fail[any](errorcode.Forbidden, "权限不足"))
			return
		}
		c.Next()
	}
}
