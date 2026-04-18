package ingestion

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
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
	}
}

// ProcessDocument fetches, parses, chunks, embeds, and indexes a document.
func (s *IngestionService) ProcessDocument(ctx context.Context, docID int64) error {
	// 1. Load document metadata from DB.
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return fmt.Errorf("ingestion: load doc %d: %w", docID, err)
	}

	// 2. 根据 KbID 推导 Milvus collection 名称（与 KBService 保持一致）。
	collectionName := knowledge.BuildCollectionName(doc.KbID)

	// 3. 构造 IngestionContext。
	//    doc.SourceLocation 存储 "s3://bucket/key" 路径（对应 Java 的 sourceLocation 字段）。
	//    doc.DocName 是文件名（对应 Java 的 docName 字段）。
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

	// 4. 构造并运行管道。
	pipeline := NewPipeline(
		NewFetcherNode(s.s3Client),
		NewParserNode(),
		NewChunkerNode(s.chunker, s.chunkSize, s.overlap),
		NewEmbedderNode(s.embedding),
		NewIndexerNode(s.milvus),
	)

	runErr := pipeline.Run(ctx, ic)

	// 5. 将 chunk 记录持久化到 MySQL。
	//    ChunkRepo.Create 接收指针，无 ctx 参数。
	//    KnowledgeChunk.Enabled 是 int（1=启用），非 bool。
	if runErr == nil {
		for _, vc := range ic.Chunks {
			if err := s.chunkRepo.Create(&knowledge.KnowledgeChunk{
				KbID:        doc.KbID,
				DocID:       docID,
				ChunkIndex:  vc.Index,
				Content:     vc.Content,
				ContentHash: hashContent(vc.Content),
				CharCount:   len([]rune(vc.Content)),
				Enabled:     1,
			}); err != nil {
				runErr = fmt.Errorf("ingestion: save chunk: %w", err)
				break
			}
		}
	}

	// 6. 更新文档状态和 chunk 数量。
	// DocRepo 接口将 UpdateStatus 和 UpdateChunkCount 拆为两个方法，均无 ctx 参数。
	status := knowledge.DocStatusSuccess
	if runErr != nil {
		status = knowledge.DocStatusFailed
	}
	_ = s.docRepo.UpdateStatus(docID, status)
	if runErr == nil {
		_ = s.docRepo.UpdateChunkCount(docID, len(ic.Chunks))
	}

	return runErr
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}
