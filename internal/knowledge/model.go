package knowledge

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// BeforeCreate 由 GORM 在 INSERT 前自动调用，赋 Snowflake ID。
// 对应 Java：@TableId(type = IdType.ASSIGN_ID)
func (k *KnowledgeBase) BeforeCreate(_ *gorm.DB) error {
	if k.ID == 0 {
		k.ID = idgen.NewID()
	}
	return nil
}

func (d *KnowledgeDocument) BeforeCreate(_ *gorm.DB) error {
	if d.ID == 0 {
		d.ID = idgen.NewID()
	}
	return nil
}

func (c *KnowledgeChunk) BeforeCreate(_ *gorm.DB) error {
	if c.ID == 0 {
		c.ID = idgen.NewID()
	}
	return nil
}

type KnowledgeBase struct {
	ID             int64          `gorm:"primaryKey"`
	Name           string         `gorm:"column:name;not null"`
	EmbeddingModel string         `gorm:"column:embedding_model"`
	CollectionName string         `gorm:"column:collection_name"`
	CreatedBy      string         `gorm:"column:created_by"`
	UpdatedBy      string         `gorm:"column:updated_by"`
	CreatedAt      time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"column:update_time;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted;index"`
}

func (KnowledgeBase) TableName() string { return "t_knowledge_base" }

// KnowledgeDocument 对应 t_knowledge_document
type KnowledgeDocument struct {
	ID              int64          `gorm:"primaryKey"`
	KbID            int64          `gorm:"column:kb_id;not null"`
	DocName         string         `gorm:"column:doc_name"`
	SourceType      string         `gorm:"column:source_type"` // file / url
	SourceLocation  string         `gorm:"column:source_location"`
	ScheduleEnabled int            `gorm:"column:schedule_enabled;default:0"`
	ScheduleCron    string         `gorm:"column:schedule_cron"`
	Enabled         int            `gorm:"column:enabled;default:1"`
	ChunkCount      int            `gorm:"column:chunk_count;default:0"`
	FileURL         string         `gorm:"column:file_url"`
	FileType        string         `gorm:"column:file_type"`
	FileSize        int64          `gorm:"column:file_size"`
	ProcessMode     string         `gorm:"column:process_mode"` // chunk / pipeline
	ChunkStrategy   string         `gorm:"column:chunk_strategy"`
	ChunkConfig     string         `gorm:"column:chunk_config"`
	PipelineID      *int64         `gorm:"column:pipeline_id"`
	Status          string         `gorm:"column:status"` // pending/running/success/failed
	CreatedBy       string         `gorm:"column:created_by"`
	UpdatedBy       string         `gorm:"column:updated_by"`
	CreatedAt       time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"column:update_time;autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted;index"`
}

func (KnowledgeDocument) TableName() string { return "t_knowledge_document" }

// KnowledgeChunk 对应 t_knowledge_chunk
type KnowledgeChunk struct {
	ID          int64          `gorm:"primaryKey"`
	KbID        int64          `gorm:"column:kb_id;not null"`
	DocID       int64          `gorm:"column:doc_id;not null"`
	ChunkIndex  int            `gorm:"column:chunk_index;default:0"`
	Content     string         `gorm:"column:content;type:text"`
	ContentHash string         `gorm:"column:content_hash"`
	CharCount   int            `gorm:"column:char_count;default:0"`
	TokenCount  int            `gorm:"column:token_count;default:0"`
	Enabled     int            `gorm:"column:enabled;default:1"`
	CreatedBy   string         `gorm:"column:created_by"`
	UpdatedBy   string         `gorm:"column:updated_by"`
	CreatedAt   time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"column:update_time;autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted;index"`
}

func (KnowledgeChunk) TableName() string { return "t_knowledge_chunk" }

// ──────────────────────── VO / 响应 ────────────────────────

type KnowledgeBaseVO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	EmbeddingModel string    `json:"embeddingModel"`
	CollectionName string    `json:"collectionName"`
	DocumentCount  int64     `json:"documentCount"`
	CreatedBy      string    `json:"createdBy"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

type KnowledgeDocumentVO struct {
	ID              string    `json:"id"`
	KbID            string    `json:"kbId"`
	DocName         string    `json:"docName"`
	SourceType      string    `json:"sourceType"`
	SourceLocation  string    `json:"sourceLocation"`
	ScheduleEnabled bool      `json:"scheduleEnabled"`
	ScheduleCron    string    `json:"scheduleCron"`
	Enabled         bool      `json:"enabled"`
	ChunkCount      int       `json:"chunkCount"`
	FileURL         string    `json:"fileUrl"`
	FileType        string    `json:"fileType"`
	FileSize        int64     `json:"fileSize"`
	ProcessMode     string    `json:"processMode"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createTime"`
	UpdatedAt       time.Time `json:"updateTime"`
}

type KnowledgeChunkVO struct {
	ID         string    `json:"id"`
	KbID       string    `json:"kbId"`
	DocID      string    `json:"docId"`
	ChunkIndex int       `json:"chunkIndex"`
	Content    string    `json:"content"`
	CharCount  int       `json:"charCount"`
	TokenCount int       `json:"tokenCount"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createTime"`
}

// PageResult 是分页响应的通用结构。
type PageResult[T any] struct {
	Total   int64 `json:"total"`
	Records []T   `json:"records"`
}
