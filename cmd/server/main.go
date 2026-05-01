package main

import (
	"context"
	"fmt"
	"os"
	"time"

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

	// 3. 初始化基础设施 client（DB / Cache / Object Storage / Vector DB）
	gormDB := db.NewMySQL(&cfg.DB)
	cache.NewRedis(&cfg.Redis)
	s3Client := storage.NewS3Client(&cfg.RustFS)
	milvusClient := vector.NewMilvus(&cfg.Milvus)

	// 4. 初始化 AI 客户端
	selectionCfg := cfg.AI.Selection.Defaults()
	healthStore := aiclient.NewHealthStore(
		selectionCfg.FailureThreshold,
		time.Duration(selectionCfg.OpenDurationMs)*time.Millisecond,
	)

	chatClients := []aiclient.ChatClient{
		aiclient.NewOpenAIChatClient().WithProvider(aiclient.ProviderOpenAI),
		aiclient.NewOpenAIChatClient().WithProvider(aiclient.ProviderOllama),
	}
	llmService, err := aiclient.NewLLMService(&cfg.AI, healthStore, chatClients)
	if err != nil {
		zap.S().Fatalf("init llm service: %v", err)
	}

	embedClients := []aiclient.EmbeddingClient{
		aiclient.NewOpenAIEmbeddingClient().WithProvider(aiclient.ProviderOpenAI),
		aiclient.NewOpenAIEmbeddingClient().WithProvider(aiclient.ProviderOllama),
	}
	embeddingService, err := aiclient.NewEmbeddingService(&cfg.AI, healthStore, embedClients)
	if err != nil {
		zap.S().Fatalf("init embedding service: %v", err)
	}

	rerankClients := []aiclient.RerankClient{
		aiclient.NewCohereRerankClient().WithProvider(aiclient.ProviderCohere),
		aiclient.NewNoopRerankClient(),
	}
	rerankService, err := aiclient.NewRerankService(&cfg.AI, healthStore, rerankClients)
	if err != nil {
		zap.S().Fatalf("init rerank service: %v", err)
	}

	// llm / rerank 在 Phase 6/7 RAG 模块接入；此处提前构造以暴露启动期配置错误
	_ = llmService
	_ = rerankService

	// 5. 初始化用户模块
	userRepo := user.NewUserRepo(gormDB)
	userSvc := user.NewUserService(userRepo)
	authSvc := user.NewAuthService(userRepo, cfg.App.JWTSecret, cfg.App.JWTExpireHours)
	authHandler := user.NewAuthHandler(authSvc)
	userHandler := user.NewUserHandler(userSvc)

	// 6. 知识库模块：Repos
	kbRepo := knowledge.NewKBRepo(gormDB)
	docRepo := knowledge.NewDocRepo(gormDB)
	chunkRepo := knowledge.NewChunkRepo(gormDB)
	scheduleRepo := knowledge.NewScheduleRepo(gormDB)
	chunkLogRepo := knowledge.NewChunkLogRepo(gormDB)

	// 7. 知识库模块：Services
	fetcher := knowledge.NewHTTPFetcher()
	kbSvc := knowledge.NewKBService(kbRepo, docRepo, s3Client, milvusClient)
	scheduleSvc := knowledge.NewScheduleService(scheduleRepo)
	docSvc := knowledge.NewDocService(docRepo, kbRepo, chunkRepo, s3Client, scheduleSvc)
	chunkSvc := knowledge.NewChunkService(chunkRepo, docRepo)
	chunkLogSvc := knowledge.NewChunkLogService(chunkLogRepo)

	// 8. Ingestion pipeline（Phase 5）
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
		chunkLogSvc,
	)

	// 9. Wire 跨模块依赖（必须在 schedule job 启动前完成，避免 race）
	docSvc.SetChunkProcessor(ingestionSvc)

	// 10. Handlers
	knowledgeKBHandler := knowledge.NewKBHandler(kbSvc)
	knowledgeDocHandler := knowledge.NewDocHandler(docSvc, chunkLogSvc)
	knowledgeChunkHandler := knowledge.NewChunkHandler(chunkSvc)

	// 11. 启动后台 schedule job（依赖已全部就绪）
	scheduleProc := knowledge.ScheduleDocProcessorFunc(func(ctx context.Context, docID int64, body []byte, fileName, contentType string) error {
		zap.L().Info("schedule process doc",
			zap.Int64("docID", docID),
			zap.String("fileName", fileName),
			zap.Int("bodySize", len(body)))
		return docSvc.StartChunk(fmt.Sprintf("%d", docID))
	})

	scheduleJob := knowledge.NewScheduleJob(scheduleRepo, docRepo, fetcher, scheduleProc, knowledge.ScheduleJobConfig{
		Owner:        fmt.Sprintf("ragent-go-%s-%d", hostname(), time.Now().UnixNano()),
		LockSeconds:  cfg.RAG.Knowledge.Schedule.LockSeconds,
		MaxFileBytes: cfg.RAG.Knowledge.Schedule.MaxFileSizeBytes,
		BatchSize:    cfg.RAG.Knowledge.Schedule.BatchSize,
		ScanInterval: time.Duration(cfg.RAG.Knowledge.Schedule.ScanDelayMs) * time.Millisecond,
	})

	jobCtx, cancelJob := context.WithCancel(context.Background())
	defer cancelJob()
	go scheduleJob.Start(jobCtx)

	// 12. Router & Server
	router := server.NewRouter(cfg.Server.BasePath, server.Deps{
		AuthHandler:           authHandler,
		UserHandler:           userHandler,
		KnowledgeKBHandler:    knowledgeKBHandler,
		KnowledgeDocHandler:   knowledgeDocHandler,
		KnowledgeChunkHandler: knowledgeChunkHandler,
		JWTSecret:             cfg.App.JWTSecret,
		DemoMode:              cfg.App.DemoMode,
	})

	server.New(cfg.Server.Port, router).Start()
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
