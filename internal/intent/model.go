package intent

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// Node 是意图树中的一个节点，对应数据库 t_intent_node 表。
//
// 一棵意图树属于一个知识库。节点可以是：
//   - KB：命中后进入知识库检索。
//   - SYSTEM：命中后可直接走系统回复，不检索知识库。
//   - MCP：命中后可交给外部工具处理。
//
// 例子：知识库 100 下可以有“产品安装”“退款政策”“闲聊问候”等节点。
// 分类器会根据用户问题给这些节点打分，后续 resolver 再决定是否检索。
type Node struct {
	ID             int64          `gorm:"primaryKey"`
	KbID           int64          `gorm:"column:kb_id;not null;index"` // 所属知识库
	ParentID       *int64         `gorm:"column:parent_id"`            // nil = 顶层节点
	Name           string         `gorm:"column:name;not null"`
	Description    string         `gorm:"column:description;type:text"`    // 供 LLM 分类参考的语义描述
	Examples       string         `gorm:"column:examples;type:text"`       // JSON 数组：示例问题（few-shot）
	Level          int            `gorm:"column:level;default:1"`          // 树形层级 1/2/3...
	Kind           Kind     `gorm:"column:kind;not null"`            // KB / SYSTEM / MCP（语义类型）
	PartitionName  string         `gorm:"column:partition_name"`           // Kind=KB 时填，对应 KB collection 下的 Milvus partition 名
	MCPToolID      string         `gorm:"column:mcp_tool_id"`              // Kind=MCP 时填，外部工具 ID
	PromptSnippet  string         `gorm:"column:prompt_snippet;type:text"` // 命中时附加到 prompt 的提示语段
	TopK           *int           `gorm:"column:top_k"`                    // 节点级 topK 覆盖（nil = 用全局默认）
	Enabled        int            `gorm:"column:enabled;default:1"`        // 0=禁用 1=启用
	SortOrder      int            `gorm:"column:sort_order;default:0"`     // 同级排序
	CreatedBy      string         `gorm:"column:created_by"`
	UpdatedBy      string         `gorm:"column:updated_by"`
	CreatedAt      time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"column:update_time;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted;index"`
}

// TableName 指定 Node 对应的数据库表名。
func (Node) TableName() string { return "t_intent_node" }

// BeforeCreate 在创建节点前补齐 ID。
//
// 如果调用方已经传入 ID，会保留调用方的值；否则使用 idgen 生成一个新 ID。
func (n *Node) BeforeCreate(_ *gorm.DB) error {
	if n.ID == 0 {
		n.ID = idgen.NewID()
	}
	return nil
}
