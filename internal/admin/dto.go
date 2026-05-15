// Package admin 实现 RAG 系统的运维侧能力：链路追踪、概览统计、运维工具。
//
// 本文件定义 trace HTTP 接口的 wire 形态。两个约定：
//   - 所有 ID 用 string，避免前端处理 int64 精度丢失
//   - 列表与详情用不同 DTO——列表不返回 question 全文等重字段，控制响应体积
package admin

import (
	"strconv"
	"time"
)

// TraceListItem 是列表页用的精简 trace 形态。
//
// 故意只带 questionPreview（截断）而非全文，也不带 rewrittenQuery /
// subQuestions——列表接口可能一次返回几十条，重字段会让响应体积爆炸。
type TraceListItem struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversationId"`
	UserID          string    `json:"userId"`
	QuestionPreview string    `json:"questionPreview"`
	ChunksCount     int       `json:"chunksCount"`
	TotalMs         int64     `json:"totalMs"`
	Success         bool      `json:"success"`
	CreatedAt       time.Time `json:"createdAt"`
}

// TraceDetail 是详情接口的完整形态，带上所有阶段耗时与错误信息。
type TraceDetail struct {
	ID               string    `json:"id"`
	ConversationID   string    `json:"conversationId"`
	UserID           string    `json:"userId"`
	Question         string    `json:"question"`
	RewrittenQuery   string    `json:"rewrittenQuery"`
	SubQuestionsJSON string    `json:"subQuestionsJson"`
	ChunksCount      int       `json:"chunksCount"`
	HistoryMs        int64     `json:"historyMs"`
	RagMs            int64     `json:"ragMs"`
	LLMMs            int64     `json:"llmMs"`
	TotalMs          int64     `json:"totalMs"`
	Success          bool      `json:"success"`
	ErrorMessage     string    `json:"errorMessage"`
	CreatedAt        time.Time `json:"createdAt"`
}

// questionPreviewMaxRunes 控制列表页问题预览的最大字符数。
const questionPreviewMaxRunes = 50

// toListItem 把存储模型转列表 wire 形态，question 按 rune 截断。
func toListItem(t TraceRecord) TraceListItem {
	preview := t.Question
	if r := []rune(preview); len(r) > questionPreviewMaxRunes {
		preview = string(r[:questionPreviewMaxRunes]) + "…"
	}
	return TraceListItem{
		ID:              strconv.FormatInt(t.ID, 10),
		ConversationID:  strconv.FormatInt(t.ConversationID, 10),
		UserID:          strconv.FormatInt(t.UserID, 10),
		QuestionPreview: preview,
		ChunksCount:     t.ChunksCount,
		TotalMs:         t.TotalMs,
		Success:         t.Success == 1,
		CreatedAt:       t.CreatedAt,
	}
}

// toListItems 批量转换。
func toListItems(list []TraceRecord) []TraceListItem {
	out := make([]TraceListItem, 0, len(list))
	for _, t := range list {
		out = append(out, toListItem(t))
	}
	return out
}

// toDetail 把存储模型转详情 wire 形态。
func toDetail(t TraceRecord) TraceDetail {
	return TraceDetail{
		ID:               strconv.FormatInt(t.ID, 10),
		ConversationID:   strconv.FormatInt(t.ConversationID, 10),
		UserID:           strconv.FormatInt(t.UserID, 10),
		Question:         t.Question,
		RewrittenQuery:   t.RewrittenQuery,
		SubQuestionsJSON: t.SubQuestionsJSON,
		ChunksCount:      t.ChunksCount,
		HistoryMs:        t.HistoryMs,
		RagMs:            t.RagMs,
		LLMMs:            t.LLMMs,
		TotalMs:          t.TotalMs,
		Success:          t.Success == 1,
		ErrorMessage:     t.ErrorMessage,
		CreatedAt:        t.CreatedAt,
	}
}
