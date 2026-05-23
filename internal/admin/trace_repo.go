// Package admin 提供 RAG 系统的运维接口与链路追踪能力。
//
// 本文件封装 TraceRecord 的数据访问。trace 记录属于写多读少的观测事实，
// 主接口只提供新增和查询能力；清理、归档等维护操作应放在专门的运维流程中。
package admin

import "gorm.io/gorm"

// TraceRepo 是 RAG 链路追踪记录的数据访问接口。
type TraceRepo interface {
	Insert(t *TraceRecord) error
	List(limit, offset int) ([]TraceRecord, int64, error)
	FindByID(id int64) (*TraceRecord, error)
}

type gormTraceRepo struct{ db *gorm.DB }

// NewTraceRepo 创建基于 GORM 的 TraceRepo 实现。
func NewTraceRepo(db *gorm.DB) TraceRepo {
	return &gormTraceRepo{db: db}
}

func (r *gormTraceRepo) Insert(t *TraceRecord) error {
	return r.db.Create(t).Error
}

// List 按 create_time 倒序返回 trace 列表，最近的记录在前。
func (r *gormTraceRepo) List(limit, offset int) ([]TraceRecord, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	var list []TraceRecord
	var total int64
	if err := r.db.Model(&TraceRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Order("create_time DESC").Limit(limit).Offset(offset).Find(&list).Error
	return list, total, err
}

func (r *gormTraceRepo) FindByID(id int64) (*TraceRecord, error) {
	var t TraceRecord
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}
