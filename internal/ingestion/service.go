package ingestion

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"go.uber.org/zap"
)

// IngestionService is the public entry point for the ingestion pipeline.
type IngestionService struct {
	s3Client  *s3.Client
	milvus    milvusclient.Client
	embedding aiclient.EmbeddingService
	docRepo   knowledge.DocRepo
	chunkRepo knowledge.ChunkRepo
	chunker   ChunkerStrategy
	chunkSize int
	overlap   int
	chunkLog  *knowledge.ChunkLogService
	tokens    aiclient.TokenCounter
}

type IngestionServiceConfig struct {
	ChunkerStrategy ChunkerStrategy // default: FixedSizeChunker{}
	ChunkSize       int             // default: 512
	Overlap         int             // default: 128
}

func NewIngestionService(
	s3Client *s3.Client,
	milvus milvusclient.Client,
	embedding aiclient.EmbeddingService,
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
		s3Client:  s3Client,
		milvus:    milvus,
		embedding: embedding,
		docRepo:   docRepo,
		chunkRepo: chunkRepo,
		chunker:   cfg.ChunkerStrategy,
		chunkSize: cfg.ChunkSize,
		overlap:   cfg.Overlap,
		chunkLog:  chunkLog,
		tokens:    tokens,
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
		Source: &DocumentSource{
			Type:     SourceTypeS3,
			Location: doc.SourceLocation,
			FileName: doc.DocName,
		},
		Status: "running",
	}

	// 4. 构造并运行管道：fetcher → parser → chunker → embedder → indexer
	pipeline := NewPipeline(
		NewFetcherNode(s.s3Client),
		NewParserNode(),
		NewChunkerNode(s.chunker, s.chunkSize, s.overlap),
		NewEmbedderNode(s.embedding),
		NewIndexerNode(s.milvus),
	)

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
