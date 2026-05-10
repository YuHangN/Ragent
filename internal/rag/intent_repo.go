package rag

import "gorm.io/gorm"

// IntentRepo 定义意图节点的数据访问能力。
//
// Handler 用它管理意图树；IntentClassifier 用它读取可分类节点。
type IntentRepo interface {
	Create(node *IntentNode) error
	Update(node *IntentNode) error
	Delete(id int64) error
	FindByID(id int64) (*IntentNode, error)
	FindByKbID(kbID int64) ([]IntentNode, error)
	// FindClassifiableByKbID 返回该 KB 下可交给 LLM 分类的节点。
	// 当前规则是：节点启用，并且没有启用的子节点。
	FindClassifiableByKbID(kbID int64) ([]IntentNode, error)
}

type gormIntentRepo struct{ db *gorm.DB }

// NewIntentRepo 创建基于 GORM 的意图节点仓库。
func NewIntentRepo(db *gorm.DB) IntentRepo { return &gormIntentRepo{db: db} }

// Create 新建一个意图节点。
func (r *gormIntentRepo) Create(node *IntentNode) error { return r.db.Create(node).Error }

// Update 保存一个意图节点的当前字段值。
func (r *gormIntentRepo) Update(node *IntentNode) error { return r.db.Save(node).Error }

// Delete 删除一个意图节点。
//
// IntentNode 配置了 DeletedAt 字段，因此这里由 GORM 执行软删除。
func (r *gormIntentRepo) Delete(id int64) error {
	return r.db.Delete(&IntentNode{}, id).Error
}

// FindByID 按节点 ID 查询单个意图节点。
func (r *gormIntentRepo) FindByID(id int64) (*IntentNode, error) {
	var node IntentNode
	if err := r.db.First(&node, id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// FindByKbID 返回某个 KB 下的全部意图节点。
//
// 结果按层级、同级排序值和 ID 排序，方便 BuildTree 组装稳定的树结构。
func (r *gormIntentRepo) FindByKbID(kbID int64) ([]IntentNode, error) {
	var nodes []IntentNode
	err := r.db.Where("kb_id = ?", kbID).
		Order("level asc, sort_order asc, id asc").
		Find(&nodes).Error
	return nodes, err
}

// FindClassifiableByKbID 返回某个 KB 下可分类的意图节点。
//
// “可分类”指 enabled=1，且没有 enabled=1 的子节点。也就是说，如果一个父节点下面
// 还有启用的子节点，分类器只看更具体的子节点。
//
// 例子：如果“售后”下面有“退款”和“换货”两个启用子节点，LLM 会分类到“退款/换货”，
// 不直接分类到“售后”。
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
