package rag

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// IntentNode 对应数据库 t_intent_node 表。
type IntentNode struct {
	ID             int64          `gorm:"primaryKey"`
	KbID           int64          `gorm:"column:kb_id;not null;index"` // 所属知识库（Kind=KB 时实际生效）
	ParentID       *int64         `gorm:"column:parent_id"`            // nil = 顶层节点
	Name           string         `gorm:"column:name;not null"`
	Description    string         `gorm:"column:description;type:text"`    // 供 LLM 分类参考的语义描述
	Examples       string         `gorm:"column:examples;type:text"`       // JSON 数组：示例问题（few-shot）
	Level          int            `gorm:"column:level;default:1"`          // 树形层级 1/2/3...
	Kind           IntentKind     `gorm:"column:kind;not null"`            // KB / SYSTEM / MCP（语义类型）
	CollectionName string         `gorm:"column:collection_name"`          // Kind=KB 时填，检索目标集合
	MCPToolID      string         `gorm:"column:mcp_tool_id"`              // Kind=MCP 时填，工具 ID（Phase 10 启用）
	PromptSnippet  string         `gorm:"column:prompt_snippet;type:text"` // 命中时附加到 prompt 的提示语段（Phase 7 启用）
	TopK           *int           `gorm:"column:top_k"`                    // 节点级 topK 覆盖（nil = 用全局默认）
	Enabled        int            `gorm:"column:enabled;default:1"`        // 0=禁用 1=启用
	SortOrder      int            `gorm:"column:sort_order;default:0"`     // 同级排序
	CreatedBy      string         `gorm:"column:created_by"`
	UpdatedBy      string         `gorm:"column:updated_by"`
	CreatedAt      time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"column:update_time;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted;index"`
}

func (IntentNode) TableName() string { return "t_intent_node" }

func (n *IntentNode) BeforeCreate(_ *gorm.DB) error {
	if n.ID == 0 {
		n.ID = idgen.NewID()
	}
	return nil
}
