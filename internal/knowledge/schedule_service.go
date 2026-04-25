package knowledge

import (
	"time"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"gorm.io/gorm"
)

const minScheduleIntervalSeconds = 60 // 最短 cron 间隔 1 分钟，防止过度刷新

type ScheduleService struct {
	repo ScheduleRepo
}

func NewScheduleService(repo ScheduleRepo) *ScheduleService {
	return &ScheduleService{repo: repo}
}

func (s *ScheduleService) Reconcile(doc *KnowledgeDocument) error {
	isURL := doc.SourceType == string(SourceTypeURL)
	hasCron := doc.ScheduleCron != ""
	enabledRequested := doc.ScheduleEnabled == 1 && isURL && hasCron

	if enabledRequested {
		if IsIntervalLessThan(doc.ScheduleCron, time.Now(), minScheduleIntervalSeconds) {
			return apperror.NewClientMsg("cron 间隔不能小于 60 秒")
		}

		next, err := NextRunTime(doc.ScheduleCron, time.Now())
		if err != nil {
			return err
		}
		schedule := &KnowledgeDocumentSchedule{
			DocID:       doc.ID,
			KbID:        doc.KbID,
			CronExpr:    doc.ScheduleCron,
			Enabled:     1,
			NextRunTime: &next,
		}
		return s.repo.Upsert(schedule)
	}

	existing, err := s.repo.FindByDocID(doc.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return apperror.NewServiceWrap("查询调度记录失败", err, nil)
	}
	existing.Enabled = 0
	existing.NextRunTime = nil
	return s.repo.UpdateAfterRun(existing)
}

// ExecPage 查询某文档的调度执行历史。
func (s *ScheduleService) ExecPage(docID int64, page, size int) (*response.PageResult[KnowledgeDocumentScheduleExec], error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 10
	}
	items, total, err := s.repo.ExecPageByDocID(docID, page, size)
	if err != nil {
		return nil, apperror.NewServiceWrap("查询失败", err, nil)
	}
	return &response.PageResult[KnowledgeDocumentScheduleExec]{Total: total, Records: items}, nil
}
