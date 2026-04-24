package knowledge

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// KnowledgeDocumentChunkLog 对应 Java KnowledgeDocumentChunkLogDO。
// 每次 Ingestion 过程记录一条，用于后台排查性能 / 错误。
type KnowledgeDocumentChunkLog struct {
	ID                int64      `gorm:"primaryKey"`
	DocID             int64      `gorm:"column:doc_id;index"`
	Status            string     `gorm:"column:status"` // running / success / failed
	ProcessMode       string     `gorm:"column:process_mode"`
	ChunkStrategy     string     `gorm:"column:chunk_strategy"`
	PipelineID        *int64     `gorm:"column:pipeline_id"`
	ExtractDuration   int64      `gorm:"column:extract_duration"`   // ms 把原始文档解析成纯文本的耗时
	ChunkDuration     int64      `gorm:"column:chunk_duration"`     // ms
	EmbeddingDuration int64      `gorm:"column:embedding_duration"` // ms
	TotalDuration     int64      `gorm:"column:total_duration"`     // ms
	ChunkCount        int        `gorm:"column:chunk_count"`
	ErrorMessage      string     `gorm:"column:error_message;size:1024"`
	StartTime         time.Time  `gorm:"column:start_time"`
	EndTime           *time.Time `gorm:"column:end_time"`
	CreatedAt         time.Time  `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"column:update_time;autoUpdateTime"`
}

func (KnowledgeDocumentChunkLog) TableName() string { return "t_knowledge_document_chunk_log" }

func (l *KnowledgeDocumentChunkLog) BeforeCreate(_ *gorm.DB) error {
	if l.ID == 0 {
		l.ID = idgen.NewID()
	}
	return nil
}

type KnowledgeDocumentChunkLogVO struct {
	ID                string     `json:"id"`
	DocID             string     `json:"docId"`
	Status            string     `json:"status"`
	ProcessMode       string     `json:"processMode"`
	ChunkStrategy     string     `json:"chunkStrategy"`
	PipelineID        string     `json:"pipelineId,omitempty"`
	ExtractDuration   int64      `json:"extractDuration"`
	ChunkDuration     int64      `json:"chunkDuration"`
	EmbeddingDuration int64      `json:"embeddingDuration"`
	TotalDuration     int64      `json:"totalDuration"`
	ChunkCount        int        `json:"chunkCount"`
	ErrorMessage      string     `json:"errorMessage,omitempty"`
	StartTime         time.Time  `json:"startTime"`
	EndTime           *time.Time `json:"endTime,omitempty"`
	CreatedAt         time.Time  `json:"createTime"`
}
