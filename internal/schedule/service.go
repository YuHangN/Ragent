package schedule

import (
	"time"

	"github.com/YuHangN/ragent-go/internal/knowledge"
	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"gorm.io/gorm"
)

const minScheduleIntervalSeconds = 60 // 最短 cron 间隔 1 分钟，防止过度刷新

// DocReconcileInput 是 Service.Reconcile 的入参——独立于 knowledge.KnowledgeDocument，
// 避免 schedule 反向依赖 knowledge。调用方（DocService）按字段拷贝传入。
type DocReconcileInput struct {
	DocID           int64
	KbID            int64
	SourceType      string // "url" 才会启用调度
	ScheduleCron    string
	ScheduleEnabled bool
}

// IsURLSource schedule_service 只关心"URL 文档"的调度。
// 这里定义一个本地常量，避免引入 knowledge 的 enums 类型。
const sourceTypeURL = "url"

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Reconcile(in DocReconcileInput) error {
	isURL := in.SourceType == sourceTypeURL
	hasCron := in.ScheduleCron != ""
	enabledRequested := in.ScheduleEnabled && isURL && hasCron

	if enabledRequested {
		if IsIntervalLessThan(in.ScheduleCron, time.Now(), minScheduleIntervalSeconds) {
			return apperror.NewClientMsg("cron 间隔不能小于 60 秒")
		}

		next, err := NextRunTime(in.ScheduleCron, time.Now())
		if err != nil {
			return err
		}
		ds := &DocumentSchedule{
			DocID:       in.DocID,
			KbID:        in.KbID,
			CronExpr:    in.ScheduleCron,
			Enabled:     1,
			NextRunTime: &next,
		}
		return s.repo.Upsert(ds)
	}

	existing, err := s.repo.FindByDocID(in.DocID)
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

// ReconcileDoc 是 knowledge.DocScheduler 接口的实现——把 *KnowledgeDocument 适配成
// 内部 DocReconcileInput。knowledge.DocService 通过这个方法回调 schedule，
// schedule 不需要持有 knowledge 类型的字段（仅在签名上引用一次）。
func (s *Service) ReconcileDoc(doc *knowledge.KnowledgeDocument) error {
	return s.Reconcile(DocReconcileInput{
		DocID:           doc.ID,
		KbID:            doc.KbID,
		SourceType:      doc.SourceType,
		ScheduleCron:    doc.ScheduleCron,
		ScheduleEnabled: doc.ScheduleEnabled == 1,
	})
}

// ExecPage 查询某文档的调度执行历史。
func (s *Service) ExecPage(docID int64, page, size int) (*response.PageResult[ExecRecord], error) {
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
	return &response.PageResult[ExecRecord]{Total: total, Records: items}, nil
}
