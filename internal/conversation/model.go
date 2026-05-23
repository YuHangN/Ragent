// Package conversation 提供 RAG Chat 的会话管理与问答链路。
//
// 本文件定义会话和消息两个 GORM 模型。每次问答会持久化用户提问与助手回答，
// 供多轮对话、历史回放和审计使用。
package conversation

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// Conversation 对应 t_conversation 表，描述一次用户 chat 会话。
//
// Title 可在首条用户消息追加时自动回填；KbIDs 以 JSON 字符串保存，避免为会话
// 知识库范围单独引入关联表。
type Conversation struct {
	ID        int64          `gorm:"primaryKey"`
	UserID    int64          `gorm:"column:user_id;not null;index"`
	Title     string         `gorm:"column:title"`
	KbIDs     string         `gorm:"column:kb_ids"`
	CreatedAt time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:update_time;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted;index"`
}

// TableName 返回会话表名。
func (Conversation) TableName() string { return "t_conversation" }

// BeforeCreate 在写入前分配 snowflake ID，避免依赖 MySQL 自增主键。
func (c *Conversation) BeforeCreate(_ *gorm.DB) error {
	if c.ID == 0 {
		c.ID = idgen.NewID()
	}
	return nil
}

// Message 对应 t_chat_message 表，每条 user、assistant 或 system 消息一行。
//
// ChunksJSON 通常只在 assistant 消息上有值，用于保存本次回答引用的 RAG 召回
// 片段元信息，便于审计和前端展开引用。user 消息只保存原始问题文本。
type Message struct {
	ID             int64          `gorm:"primaryKey"`
	ConversationID int64          `gorm:"column:conversation_id;not null;index"`
	Role           aiclient.Role  `gorm:"column:role;not null"`
	Content        string         `gorm:"column:content;type:text"`
	ChunksJSON     string         `gorm:"column:chunks_json;type:text"`
	CreatedAt      time.Time      `gorm:"column:create_time;autoCreateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted;index"`
}

// TableName 返回消息表名。
func (Message) TableName() string { return "t_chat_message" }

// BeforeCreate 在写入前分配 snowflake ID。
func (m *Message) BeforeCreate(_ *gorm.DB) error {
	if m.ID == 0 {
		m.ID = idgen.NewID()
	}
	return nil
}
