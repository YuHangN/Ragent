package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

type ChunkProcessor interface {
	Process(docID int64) error
}

// DocService 处理文档上传、分页、删除、分块触发。
type DocService struct {
	docRepo        DocRepo
	kbRepo         KBRepo
	chunkRepo      ChunkRepo
	s3             *s3.Client
	chunkProcessor ChunkProcessor // Phase 5 注入
}

func NewDocService(docRepo DocRepo, kbRepo KBRepo, chunkRepo ChunkRepo, s3Client *s3.Client) *DocService {
	return &DocService{docRepo: docRepo, kbRepo: kbRepo, chunkRepo: chunkRepo, s3: s3Client}
}

// SetChunkProcessor Phase 5 完成后由 main.go 调用注入实际处理器。
func (s *DocService) SetChunkProcessor(p ChunkProcessor) { s.chunkProcessor = p }

// Upload 保存文档元数据并把文件上传到 S3（URL 来源则只记录元数据）。
// file 为 nil 时表示 URL 类型来源。
func (s *DocService) Upload(
	kbIDStr, sourceType, sourceLocation, processMode, scheduleCron string,
	scheduleEnabled bool,
	file io.Reader, fileName string, fileSize int64,
	createdBy string,
) (*KnowledgeDocumentVO, error) {
	kbID, err := parseID(kbIDStr)
	if err != nil {
		return nil, errors.New("知识库ID非法")
	}
	kb, err := s.kbRepo.FindByID(kbID)
	if err != nil {
		return nil, errors.New("知识库不存在")
	}
	if sourceType == "" {
		if file != nil {
			sourceType = SourceTypeFile
		} else {
			sourceType = SourceTypeURL
		}
	}

	if sourceType == SourceTypeURL && strings.TrimSpace(sourceLocation) == "" {
		return nil, errors.New("来源地址不能为空")
	}

	var fileURL, fileType string
	if file != nil {
		// 上传文件到 S3，路径格式：docs/<kbID>/<name>_<timestamp>.<ext>
		ext := filepath.Ext(fileName)
		fileType = strings.TrimPrefix(strings.ToLower(ext), ".")
		objectKey := fmt.Sprintf("docs/%d/%s_%d%s",
			kbID,
			strings.TrimSuffix(fileName, ext),
			time.Now().UnixMilli(),
			ext,
		)
		if err := s.uploadToS3(kb.CollectionName, objectKey, file); err != nil {
			return nil, err
		}
		fileURL = objectKey
	}

	if processMode == "" {
		processMode = ProcessModeChunk
	}

	doc := &KnowledgeDocument{
		KbID:            kbID,
		DocName:         fileName,
		SourceType:      sourceType,
		SourceLocation:  strings.TrimSpace(sourceLocation),
		ScheduleEnabled: boolToInt(scheduleEnabled),
		ScheduleCron:    scheduleCron,
		Enabled:         1,
		ChunkCount:      0,
		FileURL:         fileURL,
		FileType:        fileType,
		FileSize:        fileSize,
		ProcessMode:     processMode,
		Status:          DocStatusPending,
		CreatedBy:       createdBy,
		UpdatedBy:       createdBy,
	}
	if err := s.docRepo.Create(doc); err != nil {
		return nil, err
	}

	return toDocVO(*doc), nil
}

// StartChunk 触发文档分块：将状态置为 running，异步调用 chunkProcessor。
func (s *DocService) StartChunk(docIDStr string) error {
	docID, err := parseID(docIDStr)
	if err != nil {
		return errors.New("文档ID非法")
	}
	if _, err := s.docRepo.FindByID(docID); err != nil {
		return errors.New("文档不存在")
	}
	if err := s.docRepo.UpdateStatus(docID, DocStatusRunning); err != nil {
		return err
	}

	if s.chunkProcessor != nil {
		go func() {
			if err := s.chunkProcessor.Process(docID); err != nil {
				zap.L().Error("chunk processing failed",
					zap.Int64("docID", docID), zap.Error(err))
				_ = s.docRepo.UpdateStatus(docID, DocStatusFailed)
			}
		}()
	}

	return nil
}

// Get 查询文档详情。
func (s *DocService) Get(docIDStr string) (*KnowledgeDocumentVO, error) {
	docID, err := parseID(docIDStr)
	if err != nil {
		return nil, errors.New("文档ID非法")
	}
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return nil, errors.New("文档不存在")
	}
	return toDocVO(*doc), nil
}

// Delete 逻辑删除文档。
func (s *DocService) Delete(docIDStr string) error {
	docID, err := parseID(docIDStr)
	if err != nil {
		return errors.New("文档ID非法")
	}
	return s.docRepo.Delete(docID)
}

// Enable 启用或禁用文档。
func (s *DocService) Enable(docIDStr string, enabled bool) error {
	docID, err := parseID(docIDStr)
	if err != nil {
		return errors.New("文档ID非法")
	}
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return errors.New("文档不存在")
	}
	doc.Enabled = boolToInt(enabled)
	return s.docRepo.Update(doc)
}

// Page 分页查询文档。
func (s *DocService) Page(kbIDStr, status, keyword string, page, size int) (*PageResult[KnowledgeDocumentVO], error) {
	kbID, err := parseID(kbIDStr)
	if err != nil {
		return nil, errors.New("知识库ID非法")
	}
	docs, total, err := s.docRepo.Page(kbID, status, keyword, page, size)
	if err != nil {
		return nil, err
	}
	records := make([]KnowledgeDocumentVO, 0, len(docs))
	for _, d := range docs {
		records = append(records, *toDocVO(d))
	}
	return &PageResult[KnowledgeDocumentVO]{Total: total, Records: records}, nil
}

// Search 文档关键字搜索。
func (s *DocService) Search(keyword string, limit int) ([]KnowledgeDocumentVO, error) {
	docs, err := s.docRepo.Search(keyword, limit)
	if err != nil {
		return nil, err
	}
	result := make([]KnowledgeDocumentVO, 0, len(docs))
	for _, d := range docs {
		result = append(result, *toDocVO(d))
	}
	return result, nil
}

func (s *DocService) uploadToS3(bucket, key string, body io.Reader) error {
	_, err := s.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	return err
}

func toDocVO(d KnowledgeDocument) *KnowledgeDocumentVO {
	return &KnowledgeDocumentVO{
		ID:              fmt.Sprintf("%d", d.ID),
		KbID:            fmt.Sprintf("%d", d.KbID),
		DocName:         d.DocName,
		SourceType:      d.SourceType,
		SourceLocation:  d.SourceLocation,
		ScheduleEnabled: d.ScheduleEnabled == 1,
		ScheduleCron:    d.ScheduleCron,
		Enabled:         d.Enabled == 1,
		ChunkCount:      d.ChunkCount,
		FileURL:         d.FileURL,
		FileType:        d.FileType,
		FileSize:        d.FileSize,
		ProcessMode:     d.ProcessMode,
		Status:          d.Status,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func parseID(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	return id, err
}
