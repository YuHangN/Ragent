package ingestion

type SourceType string

const (
	SourceTypeS3  SourceType = "s3"
	SourceTypeRaw SourceType = "raw" // 单元测试用，跳过 fetch
)

// DocumentSource describes where the raw document bytes come from.
type DocumentSource struct {
	Type     SourceType
	Location string // S3 path, e.g. "s3://kb-123/abc.pdf"
	FileName string // used for MIME detection
	Bucket   string // S3 bucket name (derived from KB collection name)
}

// VectorChunk is one text slice ready for embedding and indexing.
//
// Content 永远是原文（前端展示、存 MySQL/Milvus content 字段用）；EmbedText 是
// EnricherNode 拼出的"喂给 embedding 模型的文本"（原文 + 摘要 + 生成的问题），
// 为空时 EmbedderNode 退回用 Content。这种 embed-different-from-display 模式让
// 检索向量反映增强信息，但不需要改任何 DB schema。
//
// TargetPartition 是 ChunkRouterNode 决定的目标 Milvus partition；空值由 indexer
// 退回到 ic.PartitionName（doc 级 fallback），再退回到 "_default"。
// Metadata 在 ChunkRouterNode 里会被写入 "routing" 键，记录路由决策详情。
type VectorChunk struct {
	ChunkID         string         // snowflake ID，同时作为 MySQL 主键和 Milvus 主键
	Index           int            // position within document, 0-based
	Content         string         // raw text content（原文，存库 + 展示）
	EmbedText       string         // 用于 embedding 的增强文本；空则退回 Content
	TargetPartition string         // chunk 级 partition；空时回退到 doc 级 PartitionName
	Metadata        map[string]any // 由 enrichers / router 填充
	Embedding       []float32      // filled by EmbedderNode
}

// IngestionContext is the mutable state passed through every pipeline node.
type IngestionContext struct {
	DocID            int64
	KBCollectionName string // Milvus collection to write to
	PartitionName    string // Milvus partition to write to；空 = collection 默认分区

	Source   *DocumentSource
	RawBytes []byte
	MimeType string
	RawText  string
	Chunks   []VectorChunk

	// Populated by future EnhancerNode (Phase 6)
	EnhancedText string
	Keywords     []string
	Questions    []string
	Metadata     map[string]any

	Status string // "running" | "success" | "failed"
	Error  error
	Logs   []NodeLog
}

// NodeLog records what happened inside one node.
type NodeLog struct {
	Node       string
	Message    string
	DurationMs int64
	Success    bool
	Error      string
}

// NodeResult is the return value from a Node.Execute() call.
type NodeResult struct {
	Success        bool
	ShouldContinue bool
	Message        string
	Err            error
}

// OK returns a successful, continue result.
func OK(msg string) NodeResult {
	return NodeResult{Success: true, ShouldContinue: true, Message: msg}
}

// Fail returns a failed, stop result.
func Fail(err error) NodeResult {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return NodeResult{Success: false, ShouldContinue: false, Message: msg, Err: err}
}

// Terminate returns a successful but stop-the-pipeline result.
func Terminate(reason string) NodeResult {
	return NodeResult{Success: true, ShouldContinue: false, Message: reason}
}
