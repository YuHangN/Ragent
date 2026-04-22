package user

import (
	"github.com/YuHangN/ragent-go/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 将用户模块的路由挂载到 RouterGroup。
func RegisterRoutes(rg *gin.RouterGroup, authH *AuthHandler, userH *UserHandler, jwtSecret string) {
	auth := middleware.Auth(jwtSecret)
	adminOnly := middleware.RequireRole("admin")

	// 公开接口（不需要登录）
	rg.POST("/auth/login", authH.Login)
	rg.POST("/auth/logout", authH.Logout)

	// 需要登录的接口
	protected := rg.Group("", auth)
	{
		protected.GET("/user/me", userH.CurrentUser)
		protected.PUT("/user/password", userH.ChangePassword)
	}

	admin := rg.Group("", auth, adminOnly)
	{
		admin.GET("/users", userH.PageUsers)
		admin.POST("/users", userH.CreateUser)
		admin.PUT("/users/:id", userH.UpdateUser)
		admin.DELETE("/users/:id", userH.DeleteUser)
	}
}
