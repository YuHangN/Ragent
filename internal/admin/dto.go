// Package admin 提供 RAG 系统的运维接口与链路追踪能力。
//
// 本文件定义 trace HTTP 接口的传输结构。对外 ID 统一使用 string，避免前端
// 处理 int64 时发生精度丢失；列表与详情拆分 DTO，避免列表接口返回问题全文、
// 子问题等重字段。
package admin

import (
	"strconv"
	"time"
)

// TraceListItem 是 trace 列表页使用的轻量结构。
//
// 列表接口只返回截断后的 questionPreview，不返回问题全文、改写查询和子问题，
// 以控制批量查询时的响应体积。
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

// TraceDetail 是 trace 详情页使用的完整结构，包含阶段耗时和错误信息。
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

// questionPreviewMaxRunes 是列表页问题预览的最大 rune 数。
const questionPreviewMaxRunes = 50

// toListItem 将存储模型转换为列表传输结构，并按 rune 截断问题预览。
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

// toListItems 批量转换 trace 列表。
func toListItems(list []TraceRecord) []TraceListItem {
	out := make([]TraceListItem, 0, len(list))
	for _, t := range list {
		out = append(out, toListItem(t))
	}
	return out
}

// toDetail 将存储模型转换为详情传输结构。
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
