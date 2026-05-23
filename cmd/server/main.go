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
	"github.com/YuHangN/ragent-go/internal/admin"
	"github.com/YuHangN/ragent-go/internal/conversation"
	"github.com/YuHangN/ragent-go/internal/ingestion"
	"github.com/YuHangN/ragent-go/internal/ingestion/fetcher"
	"github.com/YuHangN/ragent-go/internal/ingestion/parser"
	"github.com/YuHangN/ragent-go/internal/intent"
	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/internal/retrieval"
	"github.com/YuHangN/ragent-go/internal/schedule"
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

	tokenCounter := aiclient.NewHeuristicTokenCounter()

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
	scheduleRepo := schedule.NewRepo(gormDB)
	chunkLogRepo := knowledge.NewChunkLogRepo(gormDB)

	// 7. 知识库模块：Services
	httpFetcher := fetcher.NewHTTPFetcher(s3Client)
	kbSvc := knowledge.NewKBService(kbRepo, docRepo, s3Client, milvusClient)
	scheduleSvc := schedule.NewService(scheduleRepo)
	docSvc := knowledge.NewDocService(docRepo, kbRepo, chunkRepo, s3Client, httpFetcher, scheduleSvc)
	chunkSvc := knowledge.NewChunkService(chunkRepo, docRepo, tokenCounter)
	chunkLogSvc := knowledge.NewChunkLogService(chunkLogRepo)

	// 8. Phase 5.5a: parser selector（多 parser）+ S3 fetcher（唯一 fetcher）
	parserSel := parser.NewDocumentParserSelector([]parser.DocumentParser{
		parser.NewTextDocumentParser(),
		parser.NewMarkdownDocumentParser(),
		parser.NewHtmlDocumentParser(),
		parser.NewTikaDocumentParser(cfg.Ingestion.Tika.URL, cfg.Ingestion.Tika.Timeout()),
	})

	s3Fetcher := fetcher.NewS3Fetcher(s3Client)

	// 8.5 intent classifier 提前构造：ingestion 的 ChunkRouterNode 和后续 RAG 链路复用同一实例。
	intentRepo := intent.NewRepo(gormDB)
	classifier := intent.NewClassifier(llmService, intentRepo)

	// 9. Ingestion pipeline（Phase 5；Phase 9 加入 LLM enrichment；Phase 10 加入 chunk router）
	ingestionSvc := ingestion.NewIngestionService(
		parserSel,
		s3Fetcher,
		milvusClient,
		embeddingService,
		llmService,
		classifier,
		docRepo,
		chunkRepo,
		ingestion.IngestionServiceConfig{
			ChunkerStrategy: ingestion.FixedSizeChunker{},
			ChunkSize:       512,
			Overlap:         128,
			Enrichment:      cfg.Ingestion.Enrichment,
			ChunkRouter:     cfg.Ingestion.ChunkRouter,
		},
		chunkLogSvc,
		tokenCounter,
	)

	// 9. Wire 跨模块依赖（必须在 schedule job 启动前完成，避免 race）
	docSvc.SetChunkProcessor(ingestionSvc)

	// 10. Handlers
	knowledgeKBHandler := knowledge.NewKBHandler(kbSvc)
	knowledgeDocHandler := knowledge.NewDocHandler(docSvc, chunkLogSvc)
	knowledgeChunkHandler := knowledge.NewChunkHandler(chunkSvc)

	// 10.5 RAG Core 服务：把 Phase 6 全套组件串成统一入口供 Phase 7 chat 调用。
	// 整个链路：QueryRewrite → intent.Resolver → MultiChannelEngine → Prompt。
	// intentRepo / classifier 已在 8.5 提前构造（与 ChunkRouterNode 复用同一实例）。
	retriever := retrieval.NewMilvusRetriever(milvusClient, embeddingService)

	rewriteSvc := retrieval.NewQueryRewriteService(llmService, cfg.RAG.QueryRewrite)
	intentResolver := intent.NewResolver(
		classifier,
		3, // 单子问题最多保留 3 个意图候选
		cfg.RAG.Search.Channels.IntentDirected.MinIntentScore, // 与 IntentDirected 通道复用同一阈值
	)

	dedupProc := &retrieval.DeduplicationProcessor{}
	rerankProc := retrieval.NewRerankProcessor(rerankService)

	intentDirectedCh := retrieval.NewIntentDirectedChannel(retriever, cfg.RAG.Search.Channels.IntentDirected)
	vectorGlobalCh := retrieval.NewVectorGlobalChannel(retriever, kbRepo, cfg.RAG.Search.Channels.VectorGlobal)

	ragEngine := retrieval.NewMultiChannelEngine(
		[]retrieval.SearchChannel{intentDirectedCh, vectorGlobalCh},
		[]retrieval.PostProcessor{dedupProc, rerankProc},
	)
	promptSvc := retrieval.NewPromptService()
	ragCoreSvc := retrieval.NewRAGCoreService(rewriteSvc, intentResolver, ragEngine, promptSvc, cfg.RAG)

	intentHandler := intent.NewHandler(intentRepo)
	testRetrieveHandler := retrieval.NewTestRetrieveHandler(ragCoreSvc)

	// 10.6 RAG 链路追踪（Phase 8 MVP）：按配置决定是否真落库。
	// cfg.RAG.Trace.Enabled=false 时用 noopRecorder，chat 主路径零开销。
	traceRepo := admin.NewTraceRepo(gormDB)
	var traceRecorder admin.TraceRecorder
	if cfg.RAG.Trace.Enabled {
		traceRecorder = admin.NewMySQLRecorder(traceRepo, zap.L())
	} else {
		traceRecorder = admin.NewNoopRecorder()
	}
	adminHandler := admin.NewHandler(traceRepo)

	// 10.7 RAG Chat（Phase 7 MVP）：在 RAG Core 之上串会话历史、LLM 调用、SSE 流式。
	convRepo := conversation.NewConversationRepo(gormDB)
	convSvc := conversation.NewConversationService(convRepo)
	chatSvc := conversation.NewChatService(convSvc, ragCoreSvc, llmService, traceRecorder)
	chatHandler := conversation.NewHandler(chatSvc, convSvc, convRepo)

	// 11. 启动后台 schedule job（依赖已全部就绪）
	scheduleProc := schedule.DocProcessorFunc(func(_ context.Context, docID int64) error {
		zap.L().Info("schedule process doc", zap.Int64("docID", docID))
		return docSvc.StartChunk(fmt.Sprintf("%d", docID))
	})

	scheduleJob := schedule.NewJob(scheduleRepo, docRepo, kbRepo, httpFetcher, scheduleProc, schedule.JobConfig{
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
		IntentHandler:         intentHandler,
		TestRetrieveHandler:   testRetrieveHandler,
		ChatHandler:           chatHandler,
		AdminHandler:          adminHandler,
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
