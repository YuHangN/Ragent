package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server 封装 http.Server，提供统一的启动和优雅关机入口。
type Server struct {
	httpServer *http.Server
}

// New 根据端口号和 Gin 引擎创建 Server 实例。
func New(port int, router *gin.Engine) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: router,
		},
	}
}

// Start 启动 HTTP 服务器，并阻塞等待 SIGINT / SIGTERM 信号。
func (s *Server) Start() {
	go func() {
		zap.L().Info("server starting", zap.String("addr", s.httpServer.Addr))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("server failed to start", zap.Error(err))
		}
	}()

	// 监听系统信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zap.L().Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		zap.L().Error("server forced to shutdown", zap.Error(err))
	}

	zap.L().Info("server exited")
}
