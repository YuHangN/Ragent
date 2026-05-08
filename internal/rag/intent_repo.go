package rag

import "gorm.io/gorm"

// IntentRepo 意图节点数据访问接口。
// 对应 Java：IntentNodeMapper
type IntentRepo interface {
	Create(node *IntentNode) error
	Update(node *IntentNode) error
	Delete(id int64) error
	FindByID(id int64) (*IntentNode, error)
	FindByKbID(kbID int64) ([]IntentNode, error)
	// FindClassifiableByKbID 返回该 KB 下所有可被 LLM 分类的"叶子"节点
	// （没有子节点 + Enabled=1）。这些节点带 CollectionName / MCPToolID
	// 等运行时所需字段，是 Classify 的输入。
	FindClassifiableByKbID(kbID int64) ([]IntentNode, error)
}

type gormIntentRepo struct{ db *gorm.DB }

func NewIntentRepo(db *gorm.DB) IntentRepo { return &gormIntentRepo{db: db} }

func (r *gormIntentRepo) Create(node *IntentNode) error { return r.db.Create(node).Error }

func (r *gormIntentRepo) Update(node *IntentNode) error { return r.db.Save(node).Error }

func (r *gormIntentRepo) Delete(id int64) error {
	return r.db.Delete(&IntentNode{}, id).Error
}

func (r *gormIntentRepo) FindByID(id int64) (*IntentNode, error) {
	var node IntentNode
	if err := r.db.First(&node, id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *gormIntentRepo) FindByKbID(kbID int64) ([]IntentNode, error) {
	var nodes []IntentNode
	err := r.db.Where("kb_id = ?", kbID).
		Order("level asc, sort_order asc, id asc").
		Find(&nodes).Error
	return nodes, err
}

// FindClassifiableByKbID 返回 enabled=1 且没有任何子节点的意图节点（=树的叶子）。
// 用 LEFT JOIN 自连接 + IS NULL 判定无子，避免 N+1 查询。
func (r *gormIntentRepo) FindClassifiableByKbID(kbID int64) ([]IntentNode, error) {
	var nodes []IntentNode
	err := r.db.Raw(`
		SELECT n.* FROM t_intent_node n
		LEFT JOIN t_intent_node c
		  ON c.parent_id = n.id AND c.deleted IS NULL AND c.enabled = 1
		WHERE n.kb_id = ? AND n.deleted IS NULL AND n.enabled = 1 AND c.id IS NULL
		ORDER BY n.sort_order ASC, n.id ASC
	`, kbID).Scan(&nodes).Error
	return nodes, err
}
