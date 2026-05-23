package ingestion

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/internal/ingestion/fetcher"
	"github.com/YuHangN/ragent-go/internal/ingestion/parser"
	"github.com/YuHangN/ragent-go/internal/intent"
	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"go.uber.org/zap"
)

// IngestionService is the public entry point for the ingestion pipeline.
type IngestionService struct {
	parserSel     *parser.DocumentParserSelector
	s3Fetcher     *fetcher.S3Fetcher
	milvus        milvusclient.Client
	embedding     aiclient.EmbeddingService
	llm           aiclient.LLMService
	classifier    *intent.Classifier
	docRepo       knowledge.DocRepo
	chunkRepo     knowledge.ChunkRepo
	chunker       ChunkerStrategy
	chunkSize     int
	overlap       int
	chunkLog      *knowledge.ChunkLogService
	tokens        aiclient.TokenCounter
	enrichmentCfg config.EnrichmentConfig
	routerCfg     config.ChunkRouterConfig
}

type IngestionServiceConfig struct {
	ChunkerStrategy ChunkerStrategy          // default: FixedSizeChunker{}
	ChunkSize       int                      // default: 512
	Overlap         int                      // default: 128
	Enrichment      config.EnrichmentConfig  // 默认零值：两个 enabled 都 false → pipeline 与 Phase 9 之前一致
	ChunkRouter     config.ChunkRouterConfig // 默认零值（Enabled=false）→ 不挂 ChunkRouterNode
}

// NewIngestionService 构造摄入服务。
//
// llm 可以传 nil——只要 Enrichment 的两个 enabled 标志都为 false，pipeline
// 不会用到它。开启任一开关又没传 llm 会在 ProcessDocument 第一次跑到对应节点
// 时崩，构造期不强校验是为了让测试可以构造一个不开 enrichment 的实例。
//
// classifier 同理——只要 ChunkRouter.Enabled=false 就不会被使用；测试构造可传 nil。
func NewIngestionService(
	parserSel *parser.DocumentParserSelector,
	s3Fetcher *fetcher.S3Fetcher,
	milvus milvusclient.Client,
	embedding aiclient.EmbeddingService,
	llm aiclient.LLMService,
	classifier *intent.Classifier,
	docRepo knowledge.DocRepo,
	chunkRepo knowledge.ChunkRepo,
	cfg IngestionServiceConfig,
	chunkLog *knowledge.ChunkLogService,
	tokens aiclient.TokenCounter,
) *IngestionService {
	if cfg.ChunkerStrategy == nil {
		cfg.ChunkerStrategy = FixedSizeChunker{}
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 512
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = 128
	}

	return &IngestionService{
		parserSel:     parserSel,
		s3Fetcher:     s3Fetcher,
		milvus:        milvus,
		embedding:     embedding,
		llm:           llm,
		classifier:    classifier,
		docRepo:       docRepo,
		chunkRepo:     chunkRepo,
		chunker:       cfg.ChunkerStrategy,
		chunkSize:     cfg.ChunkSize,
		overlap:       cfg.Overlap,
		chunkLog:      chunkLog,
		tokens:        tokens,
		enrichmentCfg: cfg.Enrichment,
		routerCfg:     cfg.ChunkRouter,
	}
}

// ProcessDocument fetches, parses, chunks, embeds, and indexes a document.
func (s *IngestionService) ProcessDocument(ctx context.Context, docID int64) error {
	start := time.Now()
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return err
	}

	logID, logErr := s.chunkLog.StartLog(docID, doc.ProcessMode, doc.ChunkStrategy)
	if logErr != nil {
		zap.L().Warn("chunklog start failed (non-fatal)", zap.Error(logErr))
	}

	chunkCount, err := s.processDocumentImpl(ctx, docID)
	elapsed := time.Since(start).Milliseconds()

	if logID > 0 {
		if err != nil {
			_ = s.chunkLog.FinishFailed(logID, err.Error(), elapsed)
		} else {
			// Phase 3.5-B 只记录总耗时；子阶段细分留到 Phase 5.5 用 FinishSuccessDetailed
			_ = s.chunkLog.FinishSuccess(logID, chunkCount, elapsed)
		}
	}

	return err
}

func (s *IngestionService) processDocumentImpl(ctx context.Context, docID int64) (int, error) {
	// 1. 加载文档元信息
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return 0, fmt.Errorf("ingestion: load doc %d: %w", docID, err)
	}

	// 2. 根据 KbID 推导 Milvus collection 名称（与 KBService 保持一致）。
	collectionName := knowledge.BuildCollectionName(doc.KbID)

	// 3. 构造 IngestionContext。
	ic := &IngestionContext{
		DocID:            docID,
		KBCollectionName: collectionName,
		PartitionName:    doc.TargetPartition, // 空 → indexer 走 _default
		Source: &DocumentSource{
			Type:     SourceTypeS3,
			Location: doc.SourceLocation,
			FileName: doc.DocName,
		},
		Status: "running",
	}

	// 4. 构造并运行管道：fetcher → parser → [enhancer] → chunker → [enricher] → [chunk_router] → embedder → indexer
	// Enhancer / Enricher 由 enrichment 配置控制；ChunkRouter 由 routerCfg 控制；都关掉时管道形状与 Phase 9 之前一致。
	nodes := []Node{
		NewFetcherNode(s.s3Fetcher),
		NewParserNode(s.parserSel),
	}
	if s.enrichmentCfg.EnhancerEnabled {
		nodes = append(nodes, NewEnhancerNode(s.llm))
	}
	nodes = append(nodes, NewChunkerNode(s.chunker, s.chunkSize, s.overlap))
	if s.enrichmentCfg.EnricherEnabled {
		nodes = append(nodes, NewEnricherNode(s.llm, s.enrichmentCfg.EnricherConcurrency()))
	}
	if s.routerCfg.Enabled && s.classifier != nil {
		nodes = append(nodes, NewChunkRouterNode(
			s.classifier,
			doc.KbID,
			ic.PartitionName,
			ChunkRouterParams{
				MinScore:    s.routerCfg.MinScore,
				Concurrency: s.routerCfg.RouterConcurrency(),
				BatchSize:   s.routerCfg.RouterBatchSize(),
				MaxRetries:  s.routerCfg.RouterMaxRetries(),
			},
		))
	}
	nodes = append(nodes,
		NewEmbedderNode(s.embedding),
		NewIndexerNode(s.milvus, IndexerParams{AutoCreatePartition: s.routerCfg.AutoCreatePartition}),
	)
	pipeline := NewPipeline(nodes...)

	runErr := pipeline.Run(ctx, ic)

	// 5. 持久化 chunk 到 MySQL（仅 pipeline 成功时）
	if runErr == nil {
		for _, vc := range ic.Chunks {
			if err := s.chunkRepo.Create(&knowledge.KnowledgeChunk{
				KbID:        doc.KbID,
				DocID:       docID,
				ChunkIndex:  vc.Index,
				Content:     vc.Content,
				ContentHash: hashContent(vc.Content),
				CharCount:   len([]rune(vc.Content)),
				TokenCount:  s.tokens.Count(vc.Content),
				Enabled:     1,
			}); err != nil {
				runErr = fmt.Errorf("ingestion: save chunk: %w", err)
				break
			}
		}
	}

	// 6. 更新文档状态和 chunk 数量
	status := knowledge.DocStatusSuccess
	if runErr != nil {
		status = knowledge.DocStatusFailed
	}
	_ = s.docRepo.UpdateStatus(docID, status.String())
	if runErr == nil {
		_ = s.docRepo.UpdateChunkCount(docID, len(ic.Chunks))
	}

	// 7. 返回给外层壳
	if runErr != nil {
		return 0, runErr
	}
	return len(ic.Chunks), nil
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}
