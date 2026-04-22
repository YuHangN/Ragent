package middleware

import (
	"net/http"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
	jwtpkg "github.com/YuHangN/ragent-go/pkg/jwt"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
)

// Auth 验证 Authorization: Bearer <token> 头，解析成功后将用户信息写入 context。
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				response.Fail[any](errorcode.Unauthorized.Code(), errorcode.Unauthorized.Message()))
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwtpkg.Parse(tokenStr, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				response.Fail[any](errorcode.Unauthorized.Code(), errorcode.Unauthorized.Message()))
			return
		}
		usercontext.Set(c, &usercontext.LoginUser{
			UserID:   claims.UserID,
			Username: claims.Username,
			Role:     claims.Role,
			Avatar:   claims.Avatar,
		})
		c.Next()
	}
}

// RequireRole 检查当前用户是否有指定角色，用于 admin 路由保护。
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := usercontext.Role(c)
		if current != role {
			c.AbortWithStatusJSON(http.StatusForbidden,
				response.Fail[any](errorcode.Forbidden.Code(), errorcode.Forbidden.Message()))
			return
		}
		c.Next()
	}
}
