package knowledge

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/idgen"
	"gorm.io/gorm"
)

// KnowledgeDocumentSchedule 对应 Java KnowledgeDocumentScheduleDO。
// 表名 t_knowledge_document_schedule。
// 每个启用定时拉取的 URL 文档对应一条记录。
type KnowledgeDocumentSchedule struct {
	ID              int64      `gorm:"primaryKey"`
	DocID           int64      `gorm:"column:doc_id;uniqueIndex"`
	KbID            int64      `gorm:"column:kb_id;index"`
	CronExpr        string     `gorm:"column:cron_expr"`
	Enabled         int        `gorm:"column:enabled;default:1"`
	NextRunTime     *time.Time `gorm:"column:next_run_time"`
	LastRunTime     *time.Time `gorm:"column:last_run_time"`
	LastSuccessTime *time.Time `gorm:"column:last_success_time"`
	LastStatus      string     `gorm:"column:last_status"`
	LastError       string     `gorm:"column:last_error;size:512"`
	LastEtag        string     `gorm:"column:last_etag"`
	LastModified    string     `gorm:"column:last_modified"`
	LastContentHash string     `gorm:"column:last_content_hash"`
	LockOwner       string     `gorm:"column:lock_owner"`
	LockUntil       *time.Time `gorm:"column:lock_until;index"`
	CreatedAt       time.Time  `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"column:update_time;autoUpdateTime"`
}

func (KnowledgeDocumentSchedule) TableName() string { return "t_knowledge_document_schedule" }

func (s *KnowledgeDocumentSchedule) BeforeCreate(_ *gorm.DB) error {
	if s.ID == 0 {
		s.ID = idgen.NewID()
	}
	return nil
}

// KnowledgeDocumentScheduleExec 对应 Java KnowledgeDocumentScheduleExecDO。
// 每次定时执行留下一条记录，用于运维排查。
type KnowledgeDocumentScheduleExec struct {
	ID           int64      `gorm:"primaryKey"`
	ScheduleID   int64      `gorm:"column:schedule_id;index"`
	DocID        int64      `gorm:"column:doc_id;index"`
	KbID         int64      `gorm:"column:kb_id"`
	Status       string     `gorm:"column:status"`
	Message      string     `gorm:"column:message;size:1024"`
	StartTime    time.Time  `gorm:"column:start_time"`
	EndTime      *time.Time `gorm:"column:end_time"`
	FileName     string     `gorm:"column:file_name"`
	FileSize     int64      `gorm:"column:file_size"`
	ContentHash  string     `gorm:"column:content_hash"`
	Etag         string     `gorm:"column:etag"`
	LastModified string     `gorm:"column:last_modified"`
	CreatedAt    time.Time  `gorm:"column:create_time;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:update_time;autoUpdateTime"`
}

func (KnowledgeDocumentScheduleExec) TableName() string { return "t_knowledge_document_schedule_exec" }

func (e *KnowledgeDocumentScheduleExec) BeforeCreate(_ *gorm.DB) error {
	if e.ID == 0 {
		e.ID = idgen.NewID()
	}
	return nil
}
