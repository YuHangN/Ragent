package knowledge

import (
	"gorm.io/gorm"
)

type ChunkRepo interface {
	Create(chunk *KnowledgeChunk) error
	FindByID(id int64) (*KnowledgeChunk, error)
	Update(chunk *KnowledgeChunk) error
	Delete(id int64) error
	PageByDocID(docID int64, enabled *int, page, size int) ([]KnowledgeChunk, int64, error)
	MaxIndexByDocID(docID int64) (int, error)
	SetEnabled(id int64, enabled int) error
	SetEnabledByDocID(docID int64, ids []int64, enabled int) error
	DeleteByDocID(docID int64) error
	FindEnabledByDocID(docID int64) ([]KnowledgeChunk, error)
}

type gormChunkRepo struct{ db *gorm.DB }

func NewChunkRepo(db *gorm.DB) ChunkRepo { return &gormChunkRepo{db: db} }

// Create INSERT INTO knowledge_chunks (...) VALUES (...)
func (r *gormChunkRepo) Create(c *KnowledgeChunk) error { return r.db.Create(c).Error }

// FindByID SELECT * FROM knowledge_chunks WHERE id = ? LIMIT 1
func (r *gormChunkRepo) FindByID(id int64) (*KnowledgeChunk, error) {
	var c KnowledgeChunk
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// Update knowledge_chunks SET ... WHERE id = ?
func (r *gormChunkRepo) Update(c *KnowledgeChunk) error { return r.db.Save(c).Error }

// Delete DELETE FROM knowledge_chunks WHERE id = ?
func (r *gormChunkRepo) Delete(id int64) error { return r.db.Delete(&KnowledgeChunk{}, id).Error }

// PageByDocID SELECT * FROM knowledge_chunks WHERE doc_id = ? [AND enabled = ?] ORDER BY chunk_index ASC LIMIT ? OFFSET ?
func (r *gormChunkRepo) PageByDocID(docID int64, enabled *int, page, size int) ([]KnowledgeChunk, int64, error) {
	var items []KnowledgeChunk
	var total int64
	q := r.db.Model(&KnowledgeChunk{}).Where("doc_id = ?", docID)
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Order("chunk_index ASC").Find(&items).Error
	return items, total, err
}

// MaxIndexByDocID SELECT COALESCE(MAX(chunk_index), -1) FROM knowledge_chunks WHERE doc_id = ?
func (r *gormChunkRepo) MaxIndexByDocID(docID int64) (int, error) {
	var maxIdx int
	err := r.db.Model(&KnowledgeChunk{}).
		Where("doc_id = ?", docID).
		Select("COALESCE(MAX(chunk_index), -1)").
		Scan(&maxIdx).Error
	return maxIdx, err
}

// SetEnabled UPDATE knowledge_chunks SET enabled = ? WHERE id = ?
func (r *gormChunkRepo) SetEnabled(id int64, enabled int) error {
	return r.db.Model(&KnowledgeChunk{}).Where("id = ?", id).Update("enabled", enabled).Error
}

// SetEnabledByDocID UPDATE knowledge_chunks SET enabled = ? WHERE doc_id = ? [AND id IN (?)]
func (r *gormChunkRepo) SetEnabledByDocID(docID int64, ids []int64, enabled int) error {
	q := r.db.Model(&KnowledgeChunk{}).Where("doc_id = ?", docID)
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	return q.Update("enabled", enabled).Error
}

// DeleteByDocID DELETE FROM knowledge_chunks WHERE doc_id = ?
func (r *gormChunkRepo) DeleteByDocID(docID int64) error {
	return r.db.Where("doc_id = ?", docID).Delete(&KnowledgeChunk{}).Error
}

// FindEnabledByDocID SELECT * FROM knowledge_chunks WHERE doc_id = ? AND enabled = 1 ORDER BY chunk_index ASC
func (r *gormChunkRepo) FindEnabledByDocID(docID int64) ([]KnowledgeChunk, error) {
	var items []KnowledgeChunk
	err := r.db.Where("doc_id = ? AND enabled = 1", docID).Order("chunk_index ASC").Find(&items).Error
	return items, err
}
