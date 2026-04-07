package server

import (
	"net/http"

	"github.com/YuHangN/ragent-go/internal/server/middleware"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

func NewRouter(basePath string) *gin.Engine {
	r := gin.New() // 不使用 gin.Default()，手动注册中间件，保持可控

	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	api := r.Group(basePath)
	registerHealthCheck(api)

	return r
}

func registerHealthCheck(rg *gin.RouterGroup) {
	rg.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Success("ok"))
	})
}
