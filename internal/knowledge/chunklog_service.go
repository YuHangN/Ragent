package knowledge

import (
	"fmt"
	"time"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
)

// ChunkLogService 管理分块过程的审计日志。
type ChunkLogService struct {
	repo ChunkLogRepo
}

func NewChunkLogService(repo ChunkLogRepo) *ChunkLogService {
	return &ChunkLogService{repo: repo}
}

// StartLog 在分块开始时插入一条 running 状态的记录，返回 logID 以便后续 Finish。
func (s *ChunkLogService) StartLog(docID int64, processMode, chunkStrategy string) (int64, error) {
	l := &KnowledgeDocumentChunkLog{
		DocID:         docID,
		Status:        string(ScheduleRunning), // 复用 running 字面量
		ProcessMode:   processMode,
		ChunkStrategy: chunkStrategy,
		StartTime:     time.Now(),
	}

	if err := s.repo.Create(l); err != nil {
		return 0, apperror.NewServiceWrap("创建分块日志失败", err, nil)
	}
	return l.ID, nil
}

// FinishSuccess 在分块完成时把记录更新为 success，带耗时和 chunkCount。
func (s *ChunkLogService) FinishSuccess(
	logID int64,
	chunkCount int,
	extractMs, chunkMs, embeddingMs int64,
) error {
	return s.finish(logID, string(ScheduleSuccess), chunkCount, "", extractMs, chunkMs, embeddingMs)
}

// FinishFailed 在分块失败时更新为 failed，记录 errorMessage。
func (s *ChunkLogService) FinishFailed(logID int64, errMsg string) error {
	return s.finish(logID, string(ScheduleFailed), 0, errMsg, 0, 0, 0)
}

func (s *ChunkLogService) finish(
	logID int64,
	status string,
	chunkCount int,
	errMsg string,
	extractMs, chunkMs, embeddingMs int64,
) error {
	l := &KnowledgeDocumentChunkLog{
		ID:                logID,
		Status:            status,
		ChunkCount:        chunkCount,
		ErrorMessage:      truncate(errMsg, 1024),
		ExtractDuration:   extractMs,
		ChunkDuration:     chunkMs,
		EmbeddingDuration: embeddingMs,
		TotalDuration:     extractMs + chunkMs + embeddingMs,
	}
	end := time.Now()
	l.EndTime = &end

	if err := s.repo.Update(l); err != nil {
		return apperror.NewServiceWrap("更新分块日志失败", err, nil)
	}
	return nil
}

// Page 分页查询某个文档的日志，按 start_time DESC。
func (s *ChunkLogService) Page(docIDStr string, page, size int) (*response.PageResult[KnowledgeDocumentChunkLogVO], error) {
	docID, err := parseID(docIDStr)
	if err != nil {
		return nil, apperror.NewClientMsg("文档ID非法")
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 10
	}

	logs, total, err := s.repo.PageByDocID(docID, page, size)
	if err != nil {
		return nil, apperror.NewServiceWrap("查询失败", err, nil)
	}
	records := make([]KnowledgeDocumentChunkLogVO, 0, len(logs))
	for _, l := range logs {
		records = append(records, toChunkLogVO(l))
	}
	return &response.PageResult[KnowledgeDocumentChunkLogVO]{Total: total, Records: records}, nil
}

func toChunkLogVO(l KnowledgeDocumentChunkLog) KnowledgeDocumentChunkLogVO {
	vo := KnowledgeDocumentChunkLogVO{
		ID:                fmt.Sprintf("%d", l.ID),
		DocID:             fmt.Sprintf("%d", l.DocID),
		Status:            l.Status,
		ProcessMode:       l.ProcessMode,
		ChunkStrategy:     l.ChunkStrategy,
		ExtractDuration:   l.ExtractDuration,
		ChunkDuration:     l.ChunkDuration,
		EmbeddingDuration: l.EmbeddingDuration,
		TotalDuration:     l.TotalDuration,
		ChunkCount:        l.ChunkCount,
		ErrorMessage:      l.ErrorMessage,
		StartTime:         l.StartTime,
		EndTime:           l.EndTime,
		CreatedAt:         l.CreatedAt,
	}
	if l.PipelineID != nil {
		vo.PipelineID = fmt.Sprintf("%d", *l.PipelineID)
	}
	return vo
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
