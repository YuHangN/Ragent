// Package intent 提供意图域：意图节点的 CRUD、LLM 分类、子问题合并。
//
// 意图（Intent）是 RAG 系统对"用户问题属于哪个业务场景"的抽象：
//   - KB 类型：命中后进入知识库检索。
//   - SYSTEM 类型：命中后直接走系统回复，不检索。
//   - MCP 类型：命中后交给外部工具处理（Phase 10）。
//
// 本包不依赖 retrieval；retrieval 反过来引用 intent.Candidate / intent.Resolver 等。
package intent

// Kind 枚举意图节点的语义类型。
type Kind string

const (
	KindKB     Kind = "KB"     // 走 RAG 检索
	KindSystem Kind = "SYSTEM" // 系统交互（不检索）
	KindMCP    Kind = "MCP"    // 调外部工具（Phase 10）
)

// Candidate 是单个意图候选——分类器的输出单元。
type Candidate struct {
	NodeID        int64
	NodeName      string
	KbID          int64
	Kind          Kind
	PartitionName string  // KB 类型生效，对应 KB collection 下的 Milvus partition 名
	MCPToolID     string  // MCP 类型生效（Phase 10 用）
	Score         float64 // 0.0–1.0
}

// SubQuestionIntent 是单子问题的意图分类结果。
type SubQuestionIntent struct {
	SubQuestion string
	Candidates  []Candidate
}

// Group 是合并所有子问题后的意图分组。
//
// AllSystemOnly 严格语义：所有子问题都仅命中"单个 SYSTEM 候选"才置位（对齐 Java
// Resolver.isSystemOnly + RAGChatServiceImpl 的 allSystemOnly 守卫）。
// 混合 SYSTEM+KB 场景（如 "你好，介绍一下产品"）AllSystemOnly=false，仍走 RAG 检索。
type Group struct {
	KbIntents     []Candidate // Kind=KB 的所有候选
	McpIntents    []Candidate // Kind=MCP 的所有候选
	AllSystemOnly bool        // 所有子问题都仅命中单个 SYSTEM 节点（用于纯系统应答短路）
}
