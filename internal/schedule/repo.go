package schedule

import (
	"time"

	"gorm.io/gorm"
)

type Repo interface {
	Upsert(s *DocumentSchedule) error
	FindByDocID(docID int64) (*DocumentSchedule, error)
	Delete(scheduleID int64) error

	// FindDue 返回 enabled=1 且 next_run_time <= now 且未被锁定的记录（至多 limit 条）。
	FindDue(now time.Time, limit int) ([]DocumentSchedule, error)
	FindByID(scheduleID int64) (*DocumentSchedule, error)

	// TryAcquireLock 用乐观锁抢占某条调度记录。返回 true 代表抢到；false 代表被其他实例占用。
	TryAcquireLock(scheduleID int64, owner string, lockUntil time.Time) (bool, error)
	// RenewLock 续约：仅当 owner 匹配时延长 lockUntil。
	RenewLock(scheduleID int64, owner string, lockUntil time.Time) error
	// ReleaseLock 释放锁（清空 lock_owner / lock_until）。
	ReleaseLock(scheduleID int64, owner string) error
	// UpdateAfterRun 统一更新 last_*、next_run_time、status、error 等字段。
	UpdateAfterRun(s *DocumentSchedule) error

	ExecCreate(e *ExecRecord) error
	ExecUpdate(e *ExecRecord) error
	ExecPageByDocID(docID int64, page, size int) ([]ExecRecord, int64, error)
}

type gormRepo struct{ db *gorm.DB }

func NewRepo(db *gorm.DB) Repo { return &gormRepo{db: db} }

func (r *gormRepo) Upsert(s *DocumentSchedule) error {
	var existing DocumentSchedule
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

func (r *gormRepo) FindByDocID(docID int64) (*DocumentSchedule, error) {
	var s DocumentSchedule
	if err := r.db.Where("doc_id = ?", docID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormRepo) Delete(scheduleID int64) error {
	return r.db.Delete(&DocumentSchedule{}, scheduleID).Error
}

func (r *gormRepo) FindDue(now time.Time, limit int) ([]DocumentSchedule, error) {
	var items []DocumentSchedule
	err := r.db.Model(&DocumentSchedule{}).
		Where("enabled = 1").
		Where("next_run_time IS NULL OR next_run_time <= ?", now).
		Where("lock_until IS NULL OR lock_until < ?", now).
		Order("next_run_time ASC").
		Limit(limit).
		Find(&items).Error

	return items, err
}

func (r *gormRepo) FindByID(scheduleID int64) (*DocumentSchedule, error) {
	var s DocumentSchedule
	if err := r.db.First(&s, scheduleID).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormRepo) TryAcquireLock(scheduleID int64, owner string, lockUntil time.Time) (bool, error) {
	now := time.Now()
	res := r.db.Model(&DocumentSchedule{}).
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

func (r *gormRepo) RenewLock(scheduleID int64, owner string, lockUntil time.Time) error {
	return r.db.Model(&DocumentSchedule{}).
		Where("id = ? AND lock_owner = ?", scheduleID, owner).
		Update("lock_until", lockUntil).Error
}

func (r *gormRepo) ReleaseLock(scheduleID int64, owner string) error {
	return r.db.Model(&DocumentSchedule{}).
		Where("id = ? AND lock_owner = ?", scheduleID, owner).
		Updates(map[string]any{
			"lock_owner": nil,
			"lock_until": nil,
		}).Error
}

func (r *gormRepo) UpdateAfterRun(s *DocumentSchedule) error {
	return r.db.Save(s).Error
}

func (r *gormRepo) ExecCreate(e *ExecRecord) error {
	return r.db.Create(e).Error
}

func (r *gormRepo) ExecUpdate(e *ExecRecord) error {
	return r.db.Save(e).Error
}

func (r *gormRepo) ExecPageByDocID(docID int64, page, size int) ([]ExecRecord, int64, error) {
	var items []ExecRecord
	var total int64

	q := r.db.Model(&ExecRecord{}).Where("doc_id = ?", docID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := q.Offset((page - 1) * size).Limit(size).Order("start_time DESC").Find(&items).Error
	return items, total, err
}
