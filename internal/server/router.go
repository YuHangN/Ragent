package server

import (
	"net/http"

	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/internal/user"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/middleware"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

type Deps struct {
	UserHandler      *user.Handler
	KnowledgeHandler *knowledge.Handler
	JWTSecret        string
}

func NewRouter(basePath string, deps Deps) *gin.Engine {
	r := gin.New() // 不使用 gin.Default()，手动注册中间件，保持可控

	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.ErrorHandler())

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound,
			response.Fail[any](errorcode.ClientError.Code(), "接口不存在: "+c.Request.URL.Path))
	})

	api := r.Group(basePath)
	registerHealthCheck(api)
	user.RegisterRoutes(api, deps.UserHandler, deps.JWTSecret)
	knowledge.RegisterRoutes(api, deps.KnowledgeHandler, middleware.Auth(deps.JWTSecret))

	return r
}

func registerHealthCheck(rg *gin.RouterGroup) {
	rg.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Success("ok"))
	})
}
