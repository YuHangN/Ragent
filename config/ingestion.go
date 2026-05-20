package config

import (
	"time"
)

type IngestionConfig struct {
	Tika        TikaConfig        `mapstructure:"tika"`
	Feishu      FeishuConfig      `mapstructure:"feishu"`
	HTTP        HTTPConfig        `mapstructure:"http"`
	Local       LocalConfig       `mapstructure:"local"`
	Enrichment  EnrichmentConfig  `mapstructure:"enrichment"`
	ChunkRouter ChunkRouterConfig `mapstructure:"chunkRouter"`
}

// EnrichmentConfig 控制 ingestion 阶段的 LLM 加工节点（Enhancer / Enricher）。
//
// 两个开关独立——可以只开文档级增强，或只开 chunk 级增强。Concurrency 限制
// Enricher 的并发 LLM 调用数，避免一篇大文档瞬间打爆 LLM 配额。
type EnrichmentConfig struct {
	EnhancerEnabled bool `mapstructure:"enhancer-enabled"`
	EnricherEnabled bool `mapstructure:"enricher-enabled"`
	Concurrency     int  `mapstructure:"concurrency"`
}

// EnricherConcurrency 返回有效并发度，未配置或非法时默认 4。
func (c EnrichmentConfig) EnricherConcurrency() int {
	if c.Concurrency <= 0 {
		return 4
	}
	return c.Concurrency
}

// ChunkRouterConfig 控制 ingestion 阶段的 chunk 级 LLM 意图路由（标配 A）。
//
// Enabled=false 时不挂 ChunkRouterNode，整篇文档共用 doc.TargetPartition——
// 与未启用 chunk routing 时行为一致。
type ChunkRouterConfig struct {
	Enabled             bool    `mapstructure:"enabled"`
	MinScore            float64 `mapstructure:"minScore"`            // 低于该分数回退 doc 级 partition
	Concurrency         int     `mapstructure:"concurrency"`         // 并发跑的 batch 数；0 → 默认 4
	BatchSize           int     `mapstructure:"batchSize"`           // 单次 LLM 调用塞多少 chunk；0 → 默认 8
	MaxRetries          int     `mapstructure:"maxRetries"`          // LLM 调用最大重试次数；0 → 默认 2
	AutoCreatePartition bool    `mapstructure:"autoCreatePartition"` // IndexerNode 写入前自动建缺失 partition
}

func (c ChunkRouterConfig) RouterConcurrency() int {
	if c.Concurrency <= 0 {
		return 4
	}
	return c.Concurrency
}

func (c ChunkRouterConfig) RouterBatchSize() int {
	if c.BatchSize <= 0 {
		return 8
	}
	return c.BatchSize
}

func (c ChunkRouterConfig) RouterMaxRetries() int {
	if c.MaxRetries <= 0 {
		return 2
	}
	return c.MaxRetries
}

type TikaConfig struct {
	URL            string `mapstructure:"url"`
	TimeoutSeconds int    `mapstructure:"timeout-seconds"`
}

func (c TikaConfig) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

type FeishuConfig struct {
	AppID     string `mapstructure:"app-id"`
	AppSecret string `mapstructure:"app-secret"`
}

type HTTPConfig struct {
	TimeoutSeconds int `mapstructure:"timeout-seconds"`
	MaxBodyMB      int `mapstructure:"max-body-mb"`
}

func (c HTTPConfig) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c HTTPConfig) MaxBodyBytes() int64 {
	if c.MaxBodyMB <= 0 {
		return 50 * 1024 * 1024
	}
	return int64(c.MaxBodyMB) * 1024 * 1024
}

type LocalConfig struct {
	AllowedRoots []string `mapstructure:"allowed-roots"`
}
