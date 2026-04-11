package knowledge

import (
	"gorm.io/gorm"
)

type KBRepo interface {
	Create(kb *KnowledgeBase) error
	FindByID(id int64) (*KnowledgeBase, error)
	Update(kb *KnowledgeBase) error
	Delete(id int64) error
	ExistsByName(name string, excludeID int64) (bool, error)
	Page(name string, page, size int) ([]KnowledgeBase, int64, error)
	DocCountByKbIDs(kbIDs []int64) (map[int64]int64, error)
}

type gormKBRepo struct{ db *gorm.DB }

func NewKBRepo(db *gorm.DB) KBRepo { return &gormKBRepo{db: db} }

// Create INSERT INTO knowledge_bases (...) VALUES (...)
func (r *gormKBRepo) Create(kb *KnowledgeBase) error { return r.db.Create(kb).Error }

// FindByID SELECT * FROM knowledge_bases WHERE id = ? LIMIT 1
func (r *gormKBRepo) FindByID(id int64) (*KnowledgeBase, error) {
	var kb KnowledgeBase
	if err := r.db.First(&kb, id).Error; err != nil {
		return nil, err
	}
	return &kb, nil
}

// Update knowledge_bases SET ... WHERE id = ?
func (r *gormKBRepo) Update(kb *KnowledgeBase) error { return r.db.Save(kb).Error }

// Delete FROM knowledge_bases WHERE id = ?
func (r *gormKBRepo) Delete(id int64) error { return r.db.Delete(&KnowledgeBase{}, id).Error }

// ExistsByName SELECT COUNT(*) FROM knowledge_bases WHERE name = ? [AND id != ?]
func (r *gormKBRepo) ExistsByName(name string, excludeID int64) (bool, error) {
	var count int64
	q := r.db.Model(&KnowledgeBase{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	return count > 0, q.Count(&count).Error
}

// Page SELECT * FROM knowledge_bases [WHERE name LIKE ?] ORDER BY update_time DESC LIMIT ? OFFSET ?
func (r *gormKBRepo) Page(name string, page, size int) ([]KnowledgeBase, int64, error) {
	var items []KnowledgeBase
	var total int64
	q := r.db.Model(&KnowledgeBase{})
	if name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Order("update_time DESC").Find(&items).Error
	return items, total, err
}

// DocCountByKbIDs SELECT kb_id, COUNT(1) AS count FROM knowledge_documents WHERE kb_id IN (?) GROUP BY kb_id
func (r *gormKBRepo) DocCountByKbIDs(kbIDs []int64) (map[int64]int64, error) {
	if len(kbIDs) == 0 {
		return map[int64]int64{}, nil
	}
	type row struct {
		KbID  int64 `gorm:"column:kb_id"`
		Count int64 `gorm:"column:count"`
	}
	var rows []row
	err := r.db.Model(&KnowledgeDocument{}).
		Select("kb_id, COUNT(1) AS count").
		Where("kb_id IN ?", kbIDs).
		Group("kb_id").
		Scan(&rows).Error
	m := make(map[int64]int64, len(rows))
	for _, row := range rows {
		m[row.KbID] = row.Count
	}
	return m, err
}
