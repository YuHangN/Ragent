// Package admin 实现 RAG 系统的运维侧能力：链路追踪、概览统计、运维工具。
//
// 本文件提供 TraceRecord 的数据访问。trace 数据特点是"写多读少 + 不可变事实"，
// 因此接口故意不暴露 Update / Delete——误改会丢失审计价值；GORM 软删除字段保留
// 是为了与项目其它表的清理策略一致。
package admin

import "gorm.io/gorm"

// TraceRepo 是 RAG 链路追踪记录的数据访问接口。
//
// 只提供 Insert（写入新 trace）+ List/FindByID（读取查询）。需要批量删除或
// 归档时由专门的 admin 工具实现，不污染主接口。
type TraceRepo interface {
	Insert(t *TraceRecord) error
	List(limit, offset int) ([]TraceRecord, int64, error)
	FindByID(id int64) (*TraceRecord, error)
}

type gormTraceRepo struct{ db *gorm.DB }

// NewTraceRepo 构造默认实现。
func NewTraceRepo(db *gorm.DB) TraceRepo {
	return &gormTraceRepo{db: db}
}

func (r *gormTraceRepo) Insert(t *TraceRecord) error {
	return r.db.Create(t).Error
}

// List 按 create_time 倒序返回 trace 列表，最近的在前。
//
// 不支持按 conversation_id / user_id 过滤——MVP 不上线运维筛选 UI，直接 SQL
// 查更灵活。后续 dashboard 阶段会加 ListByXxx 方法。
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
