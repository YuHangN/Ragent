package main

import (
	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/infra/cache"
	"github.com/YuHangN/ragent-go/infra/db"
	"github.com/YuHangN/ragent-go/internal/server"
	"github.com/YuHangN/ragent-go/internal/user"
	"github.com/YuHangN/ragent-go/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// 1. 加载配置
	cfg := config.Load()

	// 2. 初始化 Logger，并替换全局 zap.L()，让中间件里的 zap.L() 能工作
	logger.Init()
	zap.ReplaceGlobals(logger.L)

	// 3. 初始化基础设施（仅连接，不做业务）
	gormDB := db.NewMySQL(&cfg.DB)
	cache.NewRedis(&cfg.Redis)

	// 4. 初始化用户模块依赖
	userRepo := user.NewUserRepo(gormDB)
	userSvc := user.NewUserService(userRepo, cfg.App.JWTSecret, cfg.App.JWTExpireHours)
	userHandler := user.NewHandler(userSvc)

	// 5. 创建路由
	router := server.NewRouter(cfg.Server.BasePath, server.Deps{
		UserHandler: userHandler,
		JWTSecret:   cfg.App.JWTSecret,
	})

	// 6. 启动服务器（阻塞，直到收到终止信号）
	server.New(cfg.Server.Port, router).Start()
}
