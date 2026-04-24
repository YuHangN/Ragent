package knowledge

import (
	"time"
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
	ExtractDuration   int64      `gorm:"column:extract_duration"`   // ms
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
