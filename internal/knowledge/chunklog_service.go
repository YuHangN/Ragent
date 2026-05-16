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
		Status:        "running",
		ProcessMode:   processMode,
		ChunkStrategy: chunkStrategy,
		StartTime:     time.Now(),
	}

	if err := s.repo.Create(l); err != nil {
		return 0, apperror.NewServiceWrap("创建分块日志失败", err, nil)
	}
	return l.ID, nil
}

func (s *ChunkLogService) FinishSuccess(logID int64, chunkCount int, totalMs int64) error {
	return s.finish(logID, "success", chunkCount, "", 0, 0, 0, totalMs)
}

func (s *ChunkLogService) FinishSuccessDetailed(
	logID int64,
	chunkCount int,
	extractMs, chunkMs, embeddingMs int64,
) error {
	total := extractMs + chunkMs + embeddingMs
	return s.finish(logID, "success", chunkCount, "", extractMs, chunkMs, embeddingMs, total)
}

func (s *ChunkLogService) FinishFailed(logID int64, errMsg string, totalMs int64) error {
	return s.finish(logID, "failed", 0, errMsg, 0, 0, 0, totalMs)
}

func (s *ChunkLogService) finish(
	logID int64,
	status string,
	chunkCount int,
	errMsg string,
	extractMs, chunkMs, embeddingMs, totalMs int64,
) error {
	// 直接 update by id，不 select 再 save，减少一次查询
	l := &KnowledgeDocumentChunkLog{
		ID:                logID,
		Status:            status,
		ChunkCount:        chunkCount,
		ErrorMessage:      truncate(errMsg, 1024),
		ExtractDuration:   extractMs,
		ChunkDuration:     chunkMs,
		EmbeddingDuration: embeddingMs,
		TotalDuration:     totalMs, // 调用方显式传入，不再从 parts 求和
	}
	end := time.Now()
	l.EndTime = &end
	if err := s.repo.Update(l); err != nil {
		return apperror.NewServiceWrap("更新分块日志失败", err, nil)
	}
	return nil
}

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
