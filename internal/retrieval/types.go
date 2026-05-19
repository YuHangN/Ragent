package retrieval

import (
	"context"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// ──── 意图 ────────────────────────────────────────────────

type IntentKind string

const (
	IntentKindKB     IntentKind = "KB"     // 走 RAG 检索
	IntentKindSystem IntentKind = "SYSTEM" // 系统交互（不检索）
	IntentKindMCP    IntentKind = "MCP"    // 调外部工具（Phase 10）
)

// IntentCandidate 增加 Kind 字段：
type IntentCandidate struct {
	NodeID         int64
	NodeName       string
	KbID           int64
	Kind           IntentKind // 新增
	PartitionName  string     // KB 类型生效，检索目标 partition 名；MCP/SYSTEM 留空
	MCPToolID      string     // 新增（MCP 类型生效，Phase 10 用）
	Score          float64    // 0.0–1.0
}

// SubQuestionIntent 单子问题的意图分类结果
type SubQuestionIntent struct {
	SubQuestion string
	Candidates  []IntentCandidate
}

// IntentGroup 合并所有子问题后的意图分组。
// AllSystemOnly 严格语义：所有子问题都仅命中"单个 SYSTEM 候选"才置位（对齐 Java IntentResolver.isSystemOnly + RAGChatServiceImpl 的 allSystemOnly 守卫）。
// 混合 SYSTEM+KB 场景（如 "你好，介绍一下产品"）AllSystemOnly=false，仍走 RAG 检索。
type IntentGroup struct {
	KbIntents     []IntentCandidate // Kind=KB 的所有候选
	McpIntents    []IntentCandidate // Kind=MCP 的所有候选
	AllSystemOnly bool              // 所有子问题都仅命中单个 SYSTEM 节点（用于纯系统应答短路）
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
	Score          float32
	CollectionName string
}

// SearchContext 是传递给检索引擎的完整上下文。
//
// Question 是改写后的主问题，主要给 RerankProcessor 用（rerank 评估的是 chunk 对
// 整体问题的相关性，应该用主问题，不是单子问题）。
// SubQuestions 是改写产出的子问题文本列表。
// SubIntents 保留每个子问题与其意图候选的绑定关系，channel 据此做 per-sub-question
// 路由：把对的子问题送到对的集合查（findings.md "channel 检索丢弃子问题精度"）。
// IntentGroup 是 SubIntents 扁平合并后的视图，用于 SYSTEM 短路判断与外部摘要。
type SearchContext struct {
	KbIDs        []int64
	Question     string
	SubQuestions []string
	SubIntents   []SubQuestionIntent
	IntentGroup  IntentGroup
	TopK         int
	// PriorTierEmptySubQuestions：由 engine 在 priority tier 间填充。
	// 列出"前面 tier 的 KB 意图通道应该查、但实际查空"的子问题。VectorGlobal
	// 据此把这些子问题视作未覆盖触发兜底——修 Phase 6.7.1 的失败场景：
	// 文档没填 targetPartition 进了 _default，但 intent 高分让 IntentDirected
	// 去查空 partition。
	PriorTierEmptySubQuestions map[string]bool
}

// SearchChannelResult 是单个检索通道的输出。
type SearchChannelResult struct {
	ChannelName string
	Priority    int
	Chunks      []RetrievedChunk
	Confidence  float64 // 该通道结果的置信度，去重时优先保留高置信度通道的结果
	// PerSubQuestionHits 记录每个子问题在该通道实际命中的 chunk 数。
	// engine 据此推导下一 tier 的 PriorTierEmptySubQuestions，驱动 fall-through 兜底。
	PerSubQuestionHits map[string]int
}

// ──── 接口 ────────────────────────────────────────────────

// SearchChannel 是检索通道接口。
// 对应 Java：SearchChannel 接口
type SearchChannel interface {
	Name() string
	Priority() int // 数值越小优先级越高
	IsEnabled(sc SearchContext) bool
	Search(ctx context.Context, sc SearchContext) (SearchChannelResult, error)
}

// PostProcessor 是检索后处理接口。
// 对应 Java：SearchResultPostProcessor 接口
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
