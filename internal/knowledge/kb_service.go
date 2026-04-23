package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	milvclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"go.uber.org/zap"
)

// KBService 处理知识库的创建、更新、删除、查询。
type KBService struct {
	repo    KBRepo
	docRepo DocRepo
	s3      *s3.Client
	milvus  milvclient.Client
}

func NewKBService(repo KBRepo, docRepo DocRepo, s3Client *s3.Client, milvusClient milvclient.Client) *KBService {
	return &KBService{
		repo:    repo,
		docRepo: docRepo,
		s3:      s3Client,
		milvus:  milvusClient,
	}
}

// Create 创建知识库：DB 记录 → S3 bucket → Milvus collection。
func (s *KBService) Create(name, embeddingModel, operator string) (string, error) {
	name = NormalizeName(name)
	if name == "" {
		return "", errors.New("知识库名称不能为空")
	}
	exists, err := s.repo.ExistsByName(name, 0)
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.New("知识库名称已存在：" + name)
	}

	kb := &KnowledgeBase{
		Name:           name,
		EmbeddingModel: embeddingModel,
		CreatedBy:      operator,
		UpdatedBy:      operator,
	}
	if err := s.repo.Create(kb); err != nil {
		return "", err
	}

	// collection name 由 ID 生成，确保全局唯一
	kb.CollectionName = BuildCollectionName(kb.ID)
	if err := s.repo.Update(kb); err != nil {
		return "", err
	}

	if err := s.ensureS3Bucket(kb.CollectionName); err != nil {
		return "", err
	}
	if err := s.ensureMilvusCollection(kb.CollectionName); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", kb.ID), nil
}

// Rename 重命名知识库。
func (s *KBService) Rename(kbID int64, newName, operator string) error {
	newName = NormalizeName(newName)
	if newName == "" {
		return errors.New("知识库名称不能为空")
	}
	kb, err := s.repo.FindByID(kbID)
	if err != nil {
		return errors.New("知识库不存在")
	}
	exists, err := s.repo.ExistsByName(newName, kbID)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("知识库名称已存在：" + newName)
	}
	kb.Name = newName
	kb.UpdatedBy = operator
	return s.repo.Update(kb)
}

// Delete 删除知识库（要求无关联文档）。
func (s *KBService) Delete(kbID int64) error {
	count, err := s.docRepo.CountByKbID(kbID)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("知识库下仍有关联文档，无法删除")
	}
	return s.repo.Delete(kbID)
}

// GetByID 查询知识库详情。
func (s *KBService) GetByID(kbID int64) (*KnowledgeBaseVO, error) {
	kb, err := s.repo.FindByID(kbID)
	if err != nil {
		return nil, errors.New("知识库不存在")
	}
	return toKBVO(*kb, 0), nil
}

// Page 分页查询知识库（附带文档数）。
func (s *KBService) Page(name string, page, size int) (*PageResult[KnowledgeBaseVO], error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	kbs, total, err := s.repo.Page(name, page, size)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(kbs))
	for _, kb := range kbs {
		ids = append(ids, kb.ID)
	}
	docCounts, _ := s.repo.DocCountByKbIDs(ids)

	records := make([]KnowledgeBaseVO, 0, len(kbs))
	for _, kb := range kbs {
		records = append(records, *toKBVO(kb, docCounts[kb.ID]))
	}
	return &PageResult[KnowledgeBaseVO]{Total: total, Records: records}, nil
}

// ──────────────────────────────────────────────────────────
// 工具函数（导出，供测试使用）
// ──────────────────────────────────────────────────────────

// NormalizeName 去除首尾空格并合并内部连续空格。
func NormalizeName(name string) string {
	return strings.Join(strings.Fields(name), "")
}

// BuildCollectionName 根据 kb ID 生成 Milvus collection / S3 bucket 名称。
func BuildCollectionName(kbID int64) string {
	return fmt.Sprintf("kb_%d", kbID)
}

// ──────────────────────────────────────────────────────────
// 内部辅助
// ──────────────────────────────────────────────────────────

func (s *KBService) ensureS3Bucket(bucket string) error {
	_, err := s.s3.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		errStr := err.Error()
		// BucketAlreadyOwnedByYou / BucketAlreadyExists 属于幂等情况，跳过
		if strings.Contains(errStr, "BucketAlreadyOwnedByYou") ||
			strings.Contains(errStr, "BucketAlreadyExists") {
			zap.L().Warn("s3 bucket already exists, skipping", zap.String("bucket", bucket))
			return nil
		}
		return fmt.Errorf("创建 S3 bucket 失败: %w", err)
	}
	zap.L().Info("s3 bucket created", zap.String("bucket", bucket))
	return nil
}

func (s *KBService) ensureMilvusCollection(collectionName string) error {
	ctx := context.Background()
	has, err := s.milvus.HasCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("检查 Milvus collection 失败: %w", err)
	}
	if has {
		return nil
	}
	schema := &entity.Schema{
		CollectionName: collectionName,
		Fields: []*entity.Field{
			// id 为 snowflake string，与 MySQL t_knowledge_chunk.id 一致
			{Name: "id", DataType: entity.FieldTypeVarChar, PrimaryKey: true, AutoID: false,
				TypeParams: map[string]string{"max_length": "64"}},
			{Name: "doc_id", DataType: entity.FieldTypeInt64},
			{Name: "chunk_index", DataType: entity.FieldTypeInt32},
			{Name: "content", DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "65535"}},
			// bge-m3 输出 1024 维
			{Name: "embedding", DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{"dim": "1024"}},
		},
	}
	if err := s.milvus.CreateCollection(ctx, schema, 1); err != nil {
		return fmt.Errorf("创建 Milvus collection 失败: %w", err)
	}
	zap.L().Info("milvus collection created", zap.String("collection", collectionName))
	return nil
}

func toKBVO(kb KnowledgeBase, docCount int64) *KnowledgeBaseVO {
	return &KnowledgeBaseVO{
		ID:             fmt.Sprintf("%d", kb.ID),
		Name:           kb.Name,
		EmbeddingModel: kb.EmbeddingModel,
		CollectionName: kb.CollectionName,
		DocumentCount:  docCount,
		CreatedBy:      kb.CreatedBy,
		CreatedAt:      kb.CreatedAt,
		UpdatedAt:      kb.UpdatedAt,
	}
}
