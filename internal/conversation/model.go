// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 它把 Phase 6 的 RAGCoreService 检索能力与 Phase 4 的 LLMService 串起来，
// 同时把每次问答两端（user 提问 / assistant 回答）持久化到 MySQL，方便后续
// 多轮对话、历史回放和审计。
//
// 本文件只定义两个 GORM 模型：会话本体和会话下的消息记录。
package conversation

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// Conversation 对应 t_conversation 表，描述一次用户 chat 会话。
//
// Title 在首条 user 消息追加时由 ConversationService 自动用问题截断填入；
// KbIDs 序列化为 JSON 字符串，避免新增一张关联表。
type Conversation struct {
	ID        int64          `gorm:"primaryKey"`
	UserID    int64          `gorm:"column:user_id;not null;index"`
	Title     string         `gorm:"column:title"`
	KbIDs     string         `gorm:"column:kb_ids"`
	CreatedAt time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:update_time;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted;index"`
}

// TableName 返回表名。
func (Conversation) TableName() string { return "t_conversation" }

// BeforeCreate 在落库前为 ID 字段补一个 snowflake ID，避免依赖 MySQL 自增。
func (c *Conversation) BeforeCreate(_ *gorm.DB) error {
	if c.ID == 0 {
		c.ID = idgen.NewID()
	}
	return nil
}

// Message 对应 t_chat_message 表，每条 user / assistant / system 消息一行。
//
// ChunksJSON 仅在 assistant 消息上有值，存 RAG 召回的 chunk 元信息（含 chunk_id /
// 内容 / 分数 / 归属 KB），表示"这条回答是基于哪些证据写出来的"，便于审计与前端
// "展开引用"功能。user 消息只携带原始问题文本，不存检索副产物；如果未来需要做
// 检索质量分析（A/B 改写策略、召回 recall 评估），会单独走 RagTrace 表，避免污
// 染消息表。
type Message struct {
	ID             int64          `gorm:"primaryKey"`
	ConversationID int64          `gorm:"column:conversation_id;not null;index"`
	Role           aiclient.Role  `gorm:"column:role;not null"`
	Content        string         `gorm:"column:content;type:text"`
	ChunksJSON     string         `gorm:"column:chunks_json;type:text"`
	CreatedAt      time.Time      `gorm:"column:create_time;autoCreateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted;index"`
}

// TableName 返回表名。
func (Message) TableName() string { return "t_chat_message" }

// BeforeCreate 给消息分配 snowflake ID。
func (m *Message) BeforeCreate(_ *gorm.DB) error {
	if m.ID == 0 {
		m.ID = idgen.NewID()
	}
	return nil
}
