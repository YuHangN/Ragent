// Package admin 实现 RAG 系统的运维侧能力：链路追踪、概览统计、运维工具。
//
// 当前 MVP 只覆盖链路追踪——记录每次 chat 请求的耗时与结果，供后续 SQL 查询
// "哪些请求慢"、"哪些请求失败"、"哪些 chunks 召回为空"。Dashboard 高级 KPI、
// 趋势图、分位数等留待后续阶段补。
package admin

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// TraceRecord 对应 t_rag_trace 表，每次 chat 请求一条。
//
// 字段选择遵循"保守字段集"原则：先记 chat-level 汇总，不细分 rewrite/intent/
// retrieve/rerank 阶段；发现需要再加列。SubQuestionsJSON 用 JSON 字符串存，
// 避免新建关联表。Success 用 int(0/1) 而不是 bool，便于 SQL 索引和 GROUP BY。
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

// TableName 返回表名。
func (TraceRecord) TableName() string { return "t_rag_trace" }

// BeforeCreate 给 trace 分配 snowflake ID，避免依赖 MySQL 自增。
func (t *TraceRecord) BeforeCreate(_ *gorm.DB) error {
	if t.ID == 0 {
		t.ID = idgen.NewID()
	}
	return nil
}
