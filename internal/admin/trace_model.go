// Package admin 提供 RAG 系统的运维接口与链路追踪能力。
//
// 本文件定义 chat 请求的 trace 持久化模型，用于记录耗时、结果和关键检索摘要，
// 便于排查慢请求、失败请求以及召回为空等问题。
package admin

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// TraceRecord 对应 t_rag_trace 表，每次 chat 请求写入一条记录。
//
// 当前模型记录 chat 级别的汇总信息，不拆分到 rewrite、intent、retrieve、
// rerank 等更细阶段。SubQuestionsJSON 使用 JSON 字符串保存，避免为只读观测
// 数据额外引入关联表；Success 使用 int(0/1)，便于 SQL 索引和聚合统计。
type TraceRecord struct {
	ID               int64          `gorm:"primaryKey"`
	ConversationID   int64          `gorm:"column:conversation_id;index"`
	UserID           int64          `gorm:"column:user_id;index"`
	Question         string         `gorm:"column:question;type:text"`
	RewrittenQuery   string         `gorm:"column:rewritten_query;type:text"`
	SubQuestionsJSON string         `gorm:"column:sub_questions_json;type:text"`
	ChunksCount      int            `gorm:"column:chunks_count"`
	HistoryMs        int64          `gorm:"column:history_ms"`
	RagMs            int64          `gorm:"column:rag_ms"`
	LLMMs            int64          `gorm:"column:llm_ms"`
	TotalMs          int64          `gorm:"column:total_ms;index"`
	Success          int            `gorm:"column:success;index"` // 1=成功 0=失败
	ErrorMessage     string         `gorm:"column:error_message;type:text"`
	CreatedAt        time.Time      `gorm:"column:create_time;autoCreateTime;index"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted;index"`
}

// TableName 返回 trace 表名。
func (TraceRecord) TableName() string { return "t_rag_trace" }

// BeforeCreate 在写入前分配 snowflake ID，避免依赖 MySQL 自增主键。
func (t *TraceRecord) BeforeCreate(_ *gorm.DB) error {
	if t.ID == 0 {
		t.ID = idgen.NewID()
	}
	return nil
}
