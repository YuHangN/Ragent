package rag

import (
	"context"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

type IntentKind string

const (
	IntentKindRoot   IntentKind = "ROOT"
	IntentKindBranch IntentKind = "BRANCH"
	IntentKindLeaf   IntentKind = "LEAF"
)

// IntentCandidate 是 LLM 意图分类后的单条打分结果（仅 LEAF 节点）。
type IntentCandidate struct {
	NodeID         int64
	NodeName       string
	KbID           int64
	CollectionName string
	Score          float64 // 0.0–1.0
}

// ──── 查询改写 ────────────────────────────────────────────

// RewriteResult 是查询改写的输出。
type RewriteResult struct {
	RewrittenQuery string   // 改写后的主查询
	SubQuestions   []string // 拆分的子问题（至少包含改写后的主查询本身）
}

// ──── 检索 ────────────────────────────────────────────────

// RetrievedChunk 是从 Milvus 检索出的单条结果。
type RetrievedChunk struct {
	ID             string
	DocID          int64
	KbID           int64
	Content        string
	Score          float64
	CollectionName string
}

// SearchContext 是传递给检索引擎的完整上下文。
type SearchContext struct {
	KbIDs        []int64           // 要搜索的知识库 ID 列表（空=全部）
	Question     string            // 改写后的主问题
	SubQuestions []string          // 子问题列表
	Intents      []IntentCandidate // 意图分类结果
	TopK         int               // 最终返回的 chunk 数量
}

// SearchChannelResult 是单个检索通道的输出。
type SearchChannelResult struct {
	ChannelName string
	Priority    int
	Chunks      []RetrievedChunk
	Confidence  float64 // 该通道结果的置信度，去重时优先保留高置信度通道的结果
}

// ──── 接口 ────────────────────────────────────────────────

type SearchChannel interface {
	Name() string
	Priority() int // 数值越小优先级越高
	IsEnabled(sc SearchContext) bool
	Search(ctx context.Context, sc SearchContext) (SearchChannelResult, error)
}

// PostProcessor 是检索后处理接口。
type PostProcessor interface {
	Order() int // 数值越小越先执行
	Process(chunks []RetrievedChunk, results []SearchChannelResult, sc SearchContext) []RetrievedChunk
}

// ──── RAGCoreService 入口 ────────────────────────────────

// RetrieveRequest 是 RAGCoreService.Retrieve 的入参。
type RetrieveRequest struct {
	KbIDs    []int64                // 要检索的知识库列表
	Question string                 // 用户原始问题
	History  []aiclient.ChatMessage // 对话历史（用于改写时参考上下文）
	TopK     int                    // 最终返回的 chunk 数，0 时默认 5
}

// RetrieveResult 是 RAGCoreService.Retrieve 的返回结果，供 Phase 7 直接使用。
type RetrieveResult struct {
	RewrittenQuery string                 // 改写后的主查询（用于 trace）
	SubQuestions   []string               // 子问题列表（用于 trace）
	Chunks         []RetrievedChunk       // 最终排序后的 chunk 列表
	Messages       []aiclient.ChatMessage // 已构建好的 Prompt 消息序列，直接传给 LLM
}
