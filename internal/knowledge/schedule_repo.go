package knowledge

import (
	"time"

	"gorm.io/gorm"
)

type ScheduleRepo interface {
	Upsert(s *KnowledgeDocumentSchedule) error
	FindByDocID(docID int64) (*KnowledgeDocumentSchedule, error)
	Delete(scheduleID int64) error

	// FindDue 返回 enabled=1 且 next_run_time <= now 且未被锁定的记录（至多 limit 条）。
	FindDue(now time.Time, limit int) ([]KnowledgeDocumentSchedule, error)

	// TryAcquireLock 用乐观锁抢占某条调度记录。返回 true 代表抢到；false 代表被其他实例占用。
	TryAcquireLock(scheduleID int64, owner string, lockUntil time.Time) (bool, error)
	// RenewLock 续约：仅当 owner 匹配时延长 lockUntil。
	RenewLock(scheduleID int64, owner string, lockUntil time.Time) error
	// ReleaseLock 释放锁（清空 lock_owner / lock_until）。
	ReleaseLock(scheduleID int64, owner string) error
	// UpdateAfterRun 统一更新 last_*、next_run_time、status、error 等字段。
	UpdateAfterRun(s *KnowledgeDocumentSchedule) error

	ExecCreate(e *KnowledgeDocumentScheduleExec) error
	ExecUpdate(e *KnowledgeDocumentScheduleExec) error
	ExecPageByDocID(docID int64, page, size int) ([]KnowledgeDocumentScheduleExec, int64, error)
}

type gormScheduleRepo struct{ db *gorm.DB }

func NewScheduleRepo(db *gorm.DB) ScheduleRepo { return &gormScheduleRepo{db: db} }

func (r *gormScheduleRepo) Upsert(s *KnowledgeDocumentSchedule) error {
	var existing KnowledgeDocumentSchedule
	err := r.db.Where("doc_id = ?", s.DocID).First(&existing).Error
	if err != nil {
		// 如果找不到记录，则创建新记录；否则返回错误。
		if err == gorm.ErrRecordNotFound {
			return r.db.Create(s).Error
		}
		return err
	}

	// 如果记录已存在，则更新现有记录的字段（除了 ID 和 DocID），并保存。
	s.ID = existing.ID
	return r.db.Save(s).Error
}

func (r *gormScheduleRepo) FindByDocID(docID int64) (*KnowledgeDocumentSchedule, error) {
	var s KnowledgeDocumentSchedule
	if err := r.db.Where("doc_id = ?", docID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormScheduleRepo) Delete(scheduleID int64) error {
	return r.db.Delete(&KnowledgeDocumentSchedule{}, scheduleID).Error
}

func (r *gormScheduleRepo) FindDue(now time.Time, limit int) ([]KnowledgeDocumentSchedule, error) {
	var items []KnowledgeDocumentSchedule
	err := r.db.Model(&KnowledgeDocumentSchedule{}).
		Where("enabled = 1").
		Where("next_run_time IS NULL OR next_run_time <= ?", now).
		Where("lock_until IS NULL OR lock_until < ?", now).
		Order("next_run_time ASC").
		Limit(limit).
		Find(&items).Error

	return items, err
}

func (r *gormScheduleRepo) TryAcquireLock(scheduleID int64, owner string, lockUntil time.Time) (bool, error) {
	now := time.Now()
	res := r.db.Model(&KnowledgeDocumentSchedule{}).
		Where("id = ?", scheduleID).
		Where("lock_until IS NULL OR lock_until < ?", now).
		Updates(map[string]any{
			"lock_owner": owner,
			"lock_until": lockUntil,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *gormScheduleRepo) RenewLock(scheduleID int64, owner string, lockUntil time.Time) error {
	return r.db.Model(&KnowledgeDocumentSchedule{}).
		Where("id = ? AND lock_owner = ?", scheduleID, owner).
		Update("lock_until", lockUntil).Error
}

func (r *gormScheduleRepo) ReleaseLock(scheduleID int64, owner string) error {
	return r.db.Model(&KnowledgeDocumentSchedule{}).
		Where("id = ? AND lock_owner = ?", scheduleID, owner).
		Updates(map[string]any{
			"lock_owner": nil,
			"lock_until": nil,
		}).Error
}

func (r *gormScheduleRepo) UpdateAfterRun(s *KnowledgeDocumentSchedule) error {
	return r.db.Save(s).Error
}

func (r *gormScheduleRepo) ExecCreate(e *KnowledgeDocumentScheduleExec) error {
	return r.db.Create(e).Error
}

func (r *gormScheduleRepo) ExecUpdate(e *KnowledgeDocumentScheduleExec) error {
	return r.db.Save(e).Error
}

func (r *gormScheduleRepo) ExecPageByDocID(docID int64, page, size int) ([]KnowledgeDocumentScheduleExec, int64, error) {
	var items []KnowledgeDocumentScheduleExec
	var total int64

	q := r.db.Model(&KnowledgeDocumentScheduleExec{}).Where("doc_id = ?", docID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := q.Offset((page - 1) * size).Limit(size).Order("start_time DESC").Find(&items).Error
	return items, total, err
}
