package server

import (
	"net/http"

	"github.com/YuHangN/ragent-go/internal/admin"
	"github.com/YuHangN/ragent-go/internal/conversation"
	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/internal/retrieval"
	"github.com/YuHangN/ragent-go/internal/user"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/middleware"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

type Deps struct {
	AuthHandler           *user.AuthHandler
	UserHandler           *user.UserHandler
	KnowledgeKBHandler    *knowledge.KBHandler
	KnowledgeDocHandler   *knowledge.DocHandler
	KnowledgeChunkHandler *knowledge.ChunkHandler
	IntentHandler         *retrieval.IntentHandler
	ChatHandler           *conversation.Handler
	AdminHandler          *admin.Handler
	JWTSecret             string
	DemoMode              bool
}

func NewRouter(basePath string, deps Deps) *gin.Engine {
	r := gin.New() // 不使用 gin.Default()，手动注册中间件，保持可控

	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.DemoMode(deps.DemoMode))

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound,
			response.Fail[any](errorcode.ClientError.Code(), "接口不存在: "+c.Request.URL.Path))
	})

	api := r.Group(basePath)
	registerHealthCheck(api)
	user.RegisterRoutes(api, deps.AuthHandler, deps.UserHandler, deps.JWTSecret)
	knowledge.RegisterRoutes(api, deps.KnowledgeKBHandler, deps.KnowledgeDocHandler, deps.KnowledgeChunkHandler, middleware.Auth(deps.JWTSecret))

	// 意图树管理 + 调试检索；与 knowledge 同一鉴权粒度——/intent-nodes 与 /rag/test-retrieve 都
	// 会触发 LLM / 向量检索调用，必须挂 JWT 鉴权。
	ragGroup := api.Group("", middleware.Auth(deps.JWTSecret))
	retrieval.RegisterRoutes(ragGroup, deps.IntentHandler)

	// RAG Chat 主链路：/conversations CRUD + /chat + /chat/stream。
	// 全部走 JWT 鉴权——chat 触发 LLM 调用，且历史消息按用户归属隔离。
	chatGroup := api.Group("", middleware.Auth(deps.JWTSecret))
	conversation.RegisterRoutes(chatGroup, deps.ChatHandler)

	// 运维侧：/admin/traces 链路追踪查询。MVP 仅 JWT 鉴权，role 校验留后续。
	adminGroup := api.Group("", middleware.Auth(deps.JWTSecret))
	admin.RegisterRoutes(adminGroup, deps.AdminHandler)

	return r
}

func registerHealthCheck(rg *gin.RouterGroup) {
	rg.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Success("ok"))
	})
}
