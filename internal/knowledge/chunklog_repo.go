package knowledge

import (
	"gorm.io/gorm"
)

type ChunkLogRepo interface {
	Create(l *KnowledgeDocumentChunkLog) error
	Update(l *KnowledgeDocumentChunkLog) error
	PageByDocID(docID int64, page, size int) ([]KnowledgeDocumentChunkLog, int64, error)
}

type gormChunkLogRepo struct{ db *gorm.DB }

func NewChunkLogRepo(db *gorm.DB) ChunkLogRepo { return &gormChunkLogRepo{db: db} }

func (r *gormChunkLogRepo) Create(l *KnowledgeDocumentChunkLog) error {
	return r.db.Create(l).Error
}

func (r *gormChunkLogRepo) Update(l *KnowledgeDocumentChunkLog) error {
	return r.db.Save(l).Error
}

func (r *gormChunkLogRepo) PageByDocID(docID int64, page, size int) ([]KnowledgeDocumentChunkLog, int64, error) {
	var items []KnowledgeDocumentChunkLog
	var total int64
	q := r.db.Model(&KnowledgeDocumentChunkLog{}).Where("doc_id = ?", docID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Order("start_time DESC").Find(&items).Error
	return items, total, err
}
