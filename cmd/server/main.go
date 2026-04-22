package main

import (
	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/infra/cache"
	"github.com/YuHangN/ragent-go/infra/db"
	"github.com/YuHangN/ragent-go/infra/storage"
	"github.com/YuHangN/ragent-go/infra/vector"
	"github.com/YuHangN/ragent-go/internal/ingestion"
	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/internal/server"
	"github.com/YuHangN/ragent-go/internal/user"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
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

	// 4. 初始化 AI 客户端
	embeddingService, err := aiclient.NewEmbeddingService(&cfg.AI)
	if err != nil {
		zap.S().Fatalf("init embedding service: %v", err)
	}
	llmService, err := aiclient.NewLLMService(&cfg.AI)
	if err != nil {
		zap.S().Fatalf("init llm service: %v", err)
	}
	rerankService, err := aiclient.NewRerankService(&cfg.AI)
	if err != nil {
		zap.S().Fatalf("init rerank service: %v", err)
	}
	_ = embeddingService // Phase 5 ingestion pipeline
	_ = llmService       // Phase 6 RAG core
	_ = rerankService    // Phase 6 RAG retrieval

	// 5. 初始化用户模块依赖
	userRepo := user.NewUserRepo(gormDB)
	userSvc := user.NewUserService(userRepo)
	authSvc := user.NewAuthService(userRepo, cfg.App.JWTSecret, cfg.App.JWTExpireHours)
	authHandler := user.NewAuthHandler(authSvc)
	userHandler := user.NewUserHandler(userSvc)

	// 6. 初始化知识库模块依赖
	s3Client := storage.NewS3Client(&cfg.RustFS)
	milvusClient := vector.NewMilvus(&cfg.Milvus)

	kbRepo := knowledge.NewKBRepo(gormDB)
	docRepo := knowledge.NewDocRepo(gormDB)
	chunkRepo := knowledge.NewChunkRepo(gormDB)
	kbSvc := knowledge.NewKBService(kbRepo, docRepo, s3Client, milvusClient)
	docSvc := knowledge.NewDocService(docRepo, kbRepo, chunkRepo, s3Client)
	chunkSvc := knowledge.NewChunkService(chunkRepo, docRepo)

	// 7. Ingestion pipeline 依赖
	ingestionSvc := ingestion.NewIngestionService(
		s3Client,
		milvusClient,
		embeddingService,
		docRepo,
		chunkRepo,
		ingestion.IngestionServiceConfig{
			ChunkerStrategy: ingestion.FixedSizeChunker{},
			ChunkSize:       512,
			Overlap:         128,
		},
	)
	docSvc.SetChunkProcessor(ingestionSvc)

	knowledgeHandler := knowledge.NewHandler(kbSvc, docSvc, chunkSvc)

	// 7. 创建路由
	router := server.NewRouter(cfg.Server.BasePath, server.Deps{
		AuthHandler:      authHandler,
		UserHandler:      userHandler,
		KnowledgeHandler: knowledgeHandler,
		JWTSecret:        cfg.App.JWTSecret,
		DemoMode:         cfg.App.DemoMode,
	})

	// 8. 启动服务器（阻塞，直到收到终止信号）
	server.New(cfg.Server.Port, router).Start()
}
