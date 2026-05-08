package knowledge

import (
	"gorm.io/gorm"
)

type DocRepo interface {
	Create(doc *KnowledgeDocument) error
	FindByID(id int64) (*KnowledgeDocument, error)
	Update(doc *KnowledgeDocument) error
	Delete(id int64) error
	CountByKbID(kbID int64) (int64, error)
	Page(kbID int64, status, keyword string, page, size int) ([]KnowledgeDocument, int64, error)
	Search(keyword string, limit int) ([]KnowledgeDocument, error)
	UpdateStatus(id int64, status string) error
	UpdateChunkCount(id int64, delta int) error
	UpdateSourceLocation(id int64, location string) error
}

type gormDocRepo struct{ db *gorm.DB }

func NewDocRepo(db *gorm.DB) DocRepo { return &gormDocRepo{db: db} }

// Create INSERT INTO knowledge_documents (...) VALUES (...)
func (r *gormDocRepo) Create(doc *KnowledgeDocument) error { return r.db.Create(doc).Error }

// FindByID SELECT * FROM knowledge_documents WHERE id = ? LIMIT 1
func (r *gormDocRepo) FindByID(id int64) (*KnowledgeDocument, error) {
	var doc KnowledgeDocument
	if err := r.db.First(&doc, id).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

// Update knowledge_documents SET ... WHERE id = ?
func (r *gormDocRepo) Update(doc *KnowledgeDocument) error { return r.db.Save(doc).Error }

// Delete DELETE FROM knowledge_documents WHERE id = ?
func (r *gormDocRepo) Delete(id int64) error {
	return r.db.Delete(&KnowledgeDocument{}, id).Error
}

// CountByKbID SELECT COUNT(*) FROM knowledge_documents WHERE kb_id = ?
func (r *gormDocRepo) CountByKbID(kbID int64) (int64, error) {
	var count int64
	err := r.db.Model(&KnowledgeDocument{}).Where("kb_id = ?", kbID).Count(&count).Error
	return count, err
}

// Page SELECT * FROM knowledge_documents WHERE kb_id = ? [AND status = ?] [AND doc_name LIKE ?] ORDER BY update_time DESC LIMIT ? OFFSET ?
func (r *gormDocRepo) Page(kbID int64, status, keyword string, page, size int) ([]KnowledgeDocument, int64, error) {
	var items []KnowledgeDocument
	var total int64
	q := r.db.Model(&KnowledgeDocument{}).Where("kb_id = ?", kbID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		q = q.Where("doc_name LIKE ?", "%"+keyword+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Order("update_time DESC").Find(&items).Error
	return items, total, err
}

// Search SELECT * FROM knowledge_documents [WHERE doc_name LIKE ?] ORDER BY update_time DESC LIMIT ?
func (r *gormDocRepo) Search(keyword string, limit int) ([]KnowledgeDocument, error) {
	var items []KnowledgeDocument
	q := r.db.Model(&KnowledgeDocument{})
	if keyword != "" {
		q = q.Where("doc_name LIKE ?", "%"+keyword+"%")
	}
	err := q.Limit(limit).Order("update_time DESC").Find(&items).Error
	return items, err
}

// UpdateStatus UPDATE knowledge_documents SET status = ? WHERE id = ?
func (r *gormDocRepo) UpdateStatus(id int64, status string) error {
	return r.db.Model(&KnowledgeDocument{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateChunkCount UPDATE knowledge_documents SET chunk_count = chunk_count + ? WHERE id = ?
func (r *gormDocRepo) UpdateChunkCount(id int64, delta int) error {
	return r.db.Model(&KnowledgeDocument{}).Where("id = ?", id).
		UpdateColumn("chunk_count", gorm.Expr("chunk_count + ?", delta)).Error
}

// UpdateSourceLocation 更新文档的 S3 路径（schedule job 重抓后调用）。
func (r *gormDocRepo) UpdateSourceLocation(id int64, location string) error {
	return r.db.Model(&KnowledgeDocument{}).Where("id = ?", id).
		Update("source_location", location).Error
}
