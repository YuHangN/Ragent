package knowledge

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// KnowledgeDocument 对应 t_knowledge_document。
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

func (d *KnowledgeDocument) BeforeCreate(_ *gorm.DB) error {
	if d.ID == 0 {
		d.ID = idgen.NewID()
	}
	return nil
}

// KnowledgeDocumentVO 文档对外响应结构。
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
