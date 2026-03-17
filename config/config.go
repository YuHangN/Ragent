package config

// Config ── 顶层配置 ─────────────────────────────────────────────
// Config 是整个 application.yaml 的总映射入口。
// 你可以理解成：程序启动后，所有配置最终都会落到这个结构体里。
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	DB     DBConfig     `mapstructure:"db"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Milvus MilvusConfig `mapstructure:"milvus"`
	RustFS RustFSConfig `mapstructure:"rustfs"`
	AI     AIConfig     `mapstructure:"ai"`
	RAG    RAGConfig    `mapstructure:"rag"`
	App    AppConfig    `mapstructure:"app"`
}

// ── 基础设施配置 ─────────────────────────────────────────
// 这部分是服务启动时最先要用到的配置：HTTP、MySQL、Redis、Milvus、对象存储。

type ServerConfig struct {
	Port     int    `mapstructure:"port"`
	BasePath string `mapstructure:"base-path"`
}

type DBConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxOpenConns    int    `mapstructure:"max-open-conns"`
	MaxIdleConns    int    `mapstructure:"max-idle-conns"`
	ConnMaxLifetime int    `mapstructure:"conn-max-lifetime"` // 单位：秒
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type MilvusConfig struct {
	URI string `mapstructure:"uri"`
}

type RustFSConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access-key"`
	SecretKey string `mapstructure:"secret-key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use-ssl"`
}

// ── AI 配置 ─────────────────────────────────────────────
// 这部分对应 ai 节点。
// 包含：供应商信息、熔断/选择策略、聊天模型、向量模型、重排模型。

type AIConfig struct {
	Providers map[string]ProviderConfig `mapstructure:"providers"`
	Selection SelectionConfig           `mapstructure:"selection"`
	Chat      ChatModelConfig           `mapstructure:"chat"`
	Embedding EmbeddingModelConfig      `mapstructure:"embedding"`
	Rerank    RerankModelConfig         `mapstructure:"rerank"`
}

// ProviderConfig 表示一个 AI 供应商的基础信息。
// 例如 bailian / siliconflow / ollama。
type ProviderConfig struct {
	URL       string            `mapstructure:"url"`
	APIKey    string            `mapstructure:"api-key"`
	Endpoints map[string]string `mapstructure:"endpoints"`
}

// SelectionConfig 是模型选择时的公共策略。
// 后面如果你做失败切换、熔断恢复，会用到这里。
type SelectionConfig struct {
	FailureThreshold int   `mapstructure:"failure-threshold"`
	OpenDurationMs   int64 `mapstructure:"open-duration-ms"`
}

// ModelCandidate 表示一个候选模型。
// 注意：这个结构体是复用的，所以 chat / embedding / rerank 都能共用。
// SupportsThinking 主要给 chat 模型用。
// Dimension 主要给 embedding 模型用。
type ModelCandidate struct {
	ID               string `mapstructure:"id"`
	Provider         string `mapstructure:"provider"`
	Model            string `mapstructure:"model"`
	SupportsThinking bool   `mapstructure:"supports-thinking"`
	Priority         int    `mapstructure:"priority"`
	Dimension        int    `mapstructure:"dimension"` // embedding 专用
}

type ChatModelConfig struct {
	DefaultModel      string           `mapstructure:"default-model"`
	DeepThinkingModel string           `mapstructure:"deep-thinking-model"`
	Candidates        []ModelCandidate `mapstructure:"candidates"`
}

type EmbeddingModelConfig struct {
	DefaultModel string           `mapstructure:"default-model"`
	Candidates   []ModelCandidate `mapstructure:"candidates"`
}

type RerankModelConfig struct {
	DefaultModel string           `mapstructure:"default-model"`
	Candidates   []ModelCandidate `mapstructure:"candidates"`
}

// ── RAG 配置 ────────────────────────────────────────────
// 这部分是检索增强生成相关配置。
// 包括默认向量库参数、查询改写、限流、记忆、搜索通道、追踪和 MCP。

type RAGConfig struct {
	Default      DefaultVectorConfig `mapstructure:"default"`
	QueryRewrite QueryRewriteConfig  `mapstructure:"query-rewrite"`
	RateLimit    RateLimitConfig     `mapstructure:"rate-limit"`
	Memory       MemoryConfig        `mapstructure:"memory"`
	Search       SearchConfig        `mapstructure:"search"`
	Trace        TraceConfig         `mapstructure:"trace"`
	MCP          MCPConfig           `mapstructure:"mcp"`
}

// DefaultVectorConfig 表示默认知识库集合的向量参数。
type DefaultVectorConfig struct {
	CollectionName string `mapstructure:"collection-name"`
	Dimension      int    `mapstructure:"dimension"`
	MetricType     string `mapstructure:"metric-type"`
}

// QueryRewriteConfig 控制是否启用查询改写，以及改写时最多参考多少历史上下文。
type QueryRewriteConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	MaxHistoryMessages int  `mapstructure:"max-history-messages"`
	MaxHistoryChars    int  `mapstructure:"max-history-chars"`
}

type RateLimitConfig struct {
	Global GlobalRateLimitConfig `mapstructure:"global"`
}

// GlobalRateLimitConfig 是全局并发限流配置。
// 后面如果你做“同一时刻最多几个请求同时跑”，这里就会用到。
type GlobalRateLimitConfig struct {
	Enabled        bool  `mapstructure:"enabled"`
	MaxConcurrent  int   `mapstructure:"max-concurrent"`
	MaxWaitSeconds int   `mapstructure:"max-wait-seconds"`
	LeaseSeconds   int   `mapstructure:"lease-seconds"`
	PollIntervalMs int64 `mapstructure:"poll-interval-ms"`
}

// MemoryConfig 是会话记忆相关配置。
// 控制保留多少轮历史、什么时候开始总结、总结长度等。
type MemoryConfig struct {
	HistoryKeepTurns  int  `mapstructure:"history-keep-turns"`
	SummaryStartTurns int  `mapstructure:"summary-start-turns"`
	SummaryEnabled    bool `mapstructure:"summary-enabled"`
	TTLMinutes        int  `mapstructure:"ttl-minutes"`
	SummaryMaxChars   int  `mapstructure:"summary-max-chars"`
	TitleMaxLength    int  `mapstructure:"title-max-length"`
}

type SearchConfig struct {
	Channels SearchChannelsConfig `mapstructure:"channels"`
}

// SearchChannelsConfig 表示不同检索通道的配置。
// 比如全局向量检索、按意图定向检索。
type SearchChannelsConfig struct {
	VectorGlobal   VectorGlobalChannelConfig   `mapstructure:"vector-global"`
	IntentDirected IntentDirectedChannelConfig `mapstructure:"intent-directed"`
}

type VectorGlobalChannelConfig struct {
	ConfidenceThreshold float64 `mapstructure:"confidence-threshold"`
	TopKMultiplier      int     `mapstructure:"top-k-multiplier"`
}

type IntentDirectedChannelConfig struct {
	MinIntentScore float64 `mapstructure:"min-intent-score"`
	TopKMultiplier int     `mapstructure:"top-k-multiplier"`
}

// TraceConfig 控制是否记录链路追踪、错误信息最大长度等。
type TraceConfig struct {
	Enabled        bool `mapstructure:"enabled"`
	MaxErrorLength int  `mapstructure:"max-error-length"`
}

type MCPConfig struct {
	Servers []MCPServerConfig `mapstructure:"servers"`
}

type MCPServerConfig struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

// AppConfig ── 应用配置 ─────────────────────────────────────────────
// 这部分不是基础设施，而是应用自身行为配置。
// 比如 demo 模式、JWT 密钥、JWT 过期时间。
type AppConfig struct {
	DemoMode       bool   `mapstructure:"demo-mode"`
	JWTSecret      string `mapstructure:"jwt-secret"`
	JWTExpireHours int    `mapstructure:"jwt-expire-hours"`
}
