package knowledge

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// KnowledgeBase 对应 t_knowledge_base。
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

// BeforeCreate 由 GORM 在 INSERT 前自动调用，赋 Snowflake ID。
// 对应 Java：@TableId(type = IdType.ASSIGN_ID)
func (k *KnowledgeBase) BeforeCreate(_ *gorm.DB) error {
	if k.ID == 0 {
		k.ID = idgen.NewID()
	}
	return nil
}

// KnowledgeBaseVO 知识库对外响应结构。
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
