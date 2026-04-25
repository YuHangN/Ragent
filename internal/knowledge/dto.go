package knowledge

// ──────────────────────── 知识库 Request ────────────────────────

type KBCreateRequest struct {
	Name           string `json:"name" binding:"required"`
	EmbeddingModel string `json:"embeddingModel"`
}

type KBUpdateRequest struct {
	Name string `json:"name" binding:"required"`
}

type KBPageRequest struct {
	Current int    `form:"current,default=1"`
	Size    int    `form:"size,default=20"`
	Name    string `form:"name"`
}

// ──────────────────────── 文档 Request ────────────────────────

type DocUploadRequest struct {
	SourceType      string `form:"sourceType"`
	SourceLocation  string `form:"sourceLocation"`
	ProcessMode     string `form:"processMode"`
	ScheduleEnabled bool   `form:"scheduleEnabled"`
	ScheduleCron    string `form:"scheduleCron"`
	ChunkStrategy   string `form:"chunkStrategy"`
	ChunkConfig     string `form:"chunkConfig"`
}

type DocUpdateRequest struct {
	DocName         *string `json:"docName"`
	ScheduleEnabled *bool   `json:"scheduleEnabled"`
	ScheduleCron    *string `json:"scheduleCron"`
	ProcessMode     *string `json:"processMode"`
	ChunkStrategy   *string `json:"chunkStrategy"`
	ChunkConfig     *string `json:"chunkConfig"`
}

type DocPageRequest struct {
	PageNo   int    `form:"pageNo,default=1"`
	PageSize int    `form:"pageSize,default=10"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

type DocSearchRequest struct {
	Keyword string `form:"keyword"`
	Limit   int    `form:"limit,default=8"`
}

// ──────────────────────── Chunk Request ────────────────────────

// ChunkCreateRequest 对应 Java KnowledgeChunkCreateRequest。
type ChunkCreateRequest struct {
	Content string `json:"content" binding:"required"`
	Index   *int   `json:"index"`   // 可选，为空时自动取 maxIndex+1
	ChunkID string `json:"chunkId"` // 可选，为空时自动生成 Snowflake
}

type ChunkUpdateRequest struct {
	Content string `json:"content" binding:"required"`
}

type ChunkPageRequest struct {
	Current int  `form:"current,default=1"`
	Size    int  `form:"size,default=20"`
	Enabled *int `form:"enabled"`
}

type ChunkBatchRequest struct {
	IDs []int64 `json:"ids"`
}

// ChunkLogPageRequest 对应文档分块日志分页查询参数。
type ChunkLogPageRequest struct {
	PageNo   int `form:"pageNo,default=1"`
	PageSize int `form:"pageSize,default=10"`
}

// ──────────────────────── Search VO ────────────────────────

type KnowledgeDocumentSearchVO struct {
	ID      string `json:"id"`
	KbID    string `json:"kbId"`
	DocName string `json:"docName"`
	KbName  string `json:"kbName"`
}
