package user

import (
	"github.com/YuHangN/ragent-go/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 将用户模块的路由挂载到 RouterGroup。
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	auth := middleware.Auth(jwtSecret)
	adminOnly := middleware.RequireRole("admin")

	// 公开接口（不需要登录）
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/logout", h.Logout)

	// 需要登录的接口
	protected := rg.Group("", auth)
	{
		protected.GET("/user/me", h.CurrentUser)
		protected.PUT("/user/password", h.ChangePassword)
	}

	admin := rg.Group("", auth, adminOnly)
	{
		admin.GET("/users", h.PageUsers)
		admin.POST("/users", h.CreateUser)
		admin.PUT("/users/:id", h.UpdateUser)
		admin.DELETE("/users/:id", h.DeleteUser)
	}
}
