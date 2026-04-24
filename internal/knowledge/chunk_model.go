package knowledge

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// KnowledgeChunk 对应 t_knowledge_chunk。
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

func (c *KnowledgeChunk) BeforeCreate(_ *gorm.DB) error {
	if c.ID == 0 {
		c.ID = idgen.NewID()
	}
	return nil
}

// KnowledgeChunkVO Chunk 对外响应结构。
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
