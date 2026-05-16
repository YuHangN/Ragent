package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/YuHangN/ragent-go/internal/ingestion/fetcher"
	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

type ChunkProcessor interface {
	ProcessDocument(ctx context.Context, docID int64) error
}

// DocService 处理文档上传、分页、删除、分块触发。
type DocService struct {
	docRepo        DocRepo
	kbRepo         KBRepo
	chunkRepo      ChunkRepo
	s3             *s3.Client
	httpFetcher    *fetcher.HTTPFetcher
	chunkProcessor ChunkProcessor // Phase 5 注入
	schedule       *ScheduleService
}

func NewDocService(docRepo DocRepo, kbRepo KBRepo, chunkRepo ChunkRepo, s3Client *s3.Client, httpFetcher *fetcher.HTTPFetcher, scheduleSvc *ScheduleService) *DocService {
	return &DocService{docRepo: docRepo, kbRepo: kbRepo, chunkRepo: chunkRepo, s3: s3Client, httpFetcher: httpFetcher, schedule: scheduleSvc}
}

// SetChunkProcessor Phase 5 完成后由 main.go 调用注入实际处理器。
func (s *DocService) SetChunkProcessor(p ChunkProcessor) { s.chunkProcessor = p }

// Upload 保存文档元数据并把文件上传到 S3（URL 来源则只记录元数据）。
// file 为 nil 时表示 URL 类型来源。
func (s *DocService) Upload(
	kbIDStr, sourceType, sourceLocation, processMode, scheduleCron, chunkStrategy, chunkConfig, targetPartition string,
	scheduleEnabled bool,
	file io.Reader, fileName string, fileSize int64,
	operator string,
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
			sourceType = SourceTypeFile.String()
		} else {
			sourceType = SourceTypeURL.String()
		}
	}

	if sourceType == SourceTypeURL.String() && strings.TrimSpace(sourceLocation) == "" {
		return nil, errors.New("来源地址不能为空")
	}

	// Phase 5.5a 统一 S3 架构：本地上传 / URL 来源都把字节 PUT 到 S3，
	// doc.SourceLocation 永远是 s3://bucket/key 格式；URL 来源额外记录原 URL 到 OriginURL。
	var (
		fileType  string
		s3Path    string // s3://bucket/key
		originURL string
	)
	if file != nil {
		// 本地上传：直接 PUT S3。
		ext := filepath.Ext(fileName)
		fileType = strings.TrimPrefix(strings.ToLower(ext), ".")
		objectKey := fmt.Sprintf("docs/%d/%s_%d%s",
			kbID, strings.TrimSuffix(fileName, ext), time.Now().UnixMilli(), ext)
		if err := s.uploadToS3(kb.CollectionName, objectKey, file); err != nil {
			return nil, err
		}
		s3Path = fmt.Sprintf("s3://%s/%s", kb.CollectionName, objectKey)
	} else if sourceType == SourceTypeURL.String() {
		// URL 来源：下载 + PUT S3，sourceLocation 改写为 s3:// 路径。
		url := strings.TrimSpace(sourceLocation)
		originURL = url
		if fileName == "" || fileName == url {
			fileName = lastPathSegment(url)
		}
		ext := filepath.Ext(fileName)
		fileType = strings.TrimPrefix(strings.ToLower(ext), ".")
		objectKey := fmt.Sprintf("docs/%d/%s_%d%s",
			kbID, strings.TrimSuffix(fileName, ext), time.Now().UnixMilli(), ext)

		result, sp, err := s.httpFetcher.DownloadAndUploadToS3(
			context.Background(), url, kb.CollectionName, objectKey, 50*1024*1024)
		if err != nil {
			return nil, err
		}
		fileSize = int64(len(result.Body))
		s3Path = sp
	}

	if processMode == "" {
		processMode = ProcessModeChunk.String()
	}

	doc := &KnowledgeDocument{
		KbID:            kbID,
		DocName:         fileName,
		SourceType:      sourceType,
		SourceLocation:  s3Path, // 永远是 s3://bucket/key
		OriginURL:       originURL,
		ScheduleEnabled: boolToInt(scheduleEnabled),
		ScheduleCron:    scheduleCron,
		Enabled:         1,
		ChunkCount:      0,
		FileType:        fileType,
		FileSize:        fileSize,
		ProcessMode:     processMode,
		ChunkStrategy:   chunkStrategy,
		ChunkConfig:     chunkConfig,
		Status:          DocStatusPending.String(),
		TargetPartition: targetPartition,
		CreatedBy:       operator,
		UpdatedBy:       operator,
	}
	if err := s.docRepo.Create(doc); err != nil {
		return nil, err
	}

	if err := s.schedule.Reconcile(doc); err != nil {
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
	if err := s.docRepo.UpdateStatus(docID, DocStatusRunning.String()); err != nil {
		return err
	}

	if s.chunkProcessor != nil {
		go func() {
			if err := s.chunkProcessor.ProcessDocument(context.Background(), docID); err != nil {
				zap.L().Error("chunk processing failed",
					zap.Int64("docID", docID), zap.Error(err))
				_ = s.docRepo.UpdateStatus(docID, DocStatusFailed.String())
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

// Update 更新文档元信息（docName / schedule / processMode / chunk 配置）。
func (s *DocService) Update(docIDStr string, req DocUpdateRequest, operator string) error {
	docID, err := parseID(docIDStr)
	if err != nil {
		return apperror.NewClientMsg("文档ID非法")
	}
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return apperror.NewClientMsg("文档不存在")
	}
	if req.DocName != nil {
		if strings.TrimSpace(*req.DocName) == "" {
			return apperror.NewClientMsg("文档名不能为空")
		}
		doc.DocName = *req.DocName
	}
	if req.ScheduleEnabled != nil {
		doc.ScheduleEnabled = boolToInt(*req.ScheduleEnabled)
	}
	if req.ScheduleCron != nil {
		doc.ScheduleCron = *req.ScheduleCron
	}
	if req.ProcessMode != nil {
		mode, err := NormalizeProcessMode(*req.ProcessMode)
		if err != nil {
			return err
		}
		doc.ProcessMode = mode.String()
	}
	if req.ChunkStrategy != nil {
		doc.ChunkStrategy = *req.ChunkStrategy
	}
	if req.ChunkConfig != nil {
		doc.ChunkConfig = *req.ChunkConfig
	}
	if req.TargetPartition != nil {
		doc.TargetPartition = *req.TargetPartition
	}
	doc.UpdatedBy = operator

	if err := s.docRepo.Update(doc); err != nil {
		return apperror.NewServiceWrap("更新文档失败", err, nil)
	}
	return nil
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
func (s *DocService) Enable(docIDStr string, enabled bool, operator string) error {
	docID, err := parseID(docIDStr)
	if err != nil {
		return errors.New("文档ID非法")
	}
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return errors.New("文档不存在")
	}
	doc.Enabled = boolToInt(enabled)
	doc.UpdatedBy = operator
	return s.docRepo.Update(doc)
}

// Page 分页查询文档。
func (s *DocService) Page(kbIDStr, status, keyword string, page, size int) (*response.PageResult[KnowledgeDocumentVO], error) {
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
	return &response.PageResult[KnowledgeDocumentVO]{Total: total, Records: records}, nil
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

// lastPathSegment 从 URL 的最后一段路径推文件名。
func lastPathSegment(url string) string {
	if idx := strings.LastIndex(url, "/"); idx >= 0 && idx < len(url)-1 {
		name := url[idx+1:]
		if q := strings.Index(name, "?"); q >= 0 {
			name = name[:q]
		}
		return name
	}
	return url
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
		OriginURL:       d.OriginURL,
		ScheduleEnabled: d.ScheduleEnabled == 1,
		ScheduleCron:    d.ScheduleCron,
		Enabled:         d.Enabled == 1,
		ChunkCount:      d.ChunkCount,
		FileType:        d.FileType,
		FileSize:        d.FileSize,
		ProcessMode:     d.ProcessMode,
		Status:          d.Status,
		TargetPartition: d.TargetPartition,
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
