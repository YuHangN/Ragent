package knowledge

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type ScheduleDocProcessor interface {
	Process(ctx context.Context, docID int64, body []byte, fileName, contentType string) error
}

// ScheduleDocProcessorFunc 函数适配。
type ScheduleDocProcessorFunc func(ctx context.Context, docID int64, body []byte, fileName, contentType string) error

func (f ScheduleDocProcessorFunc) Process(ctx context.Context, docID int64, body []byte, fileName, contentType string) error {
	return f(ctx, docID, body, fileName, contentType)
}

// ScheduleJobConfig 配置。
type ScheduleJobConfig struct {
	Owner        string        // 实例 ID，启动时用 hostname + uuid
	LockSeconds  int64         // 锁租约秒数，默认 900
	MaxFileBytes int64         // 远程文件大小上限
	BatchSize    int           // 单次 tick 拉取的调度数量
	ScanInterval time.Duration // tick 周期，默认 10s
}

// ScheduleJob 定时任务的核心。对齐 Java KnowledgeDocumentScheduleJob。
type ScheduleJob struct {
	repo    ScheduleRepo
	docRepo DocRepo
	fetcher *HTTPFetcher
	proc    ScheduleDocProcessor
	cfg     ScheduleJobConfig

	// DocLookup 允许测试或集成时注入自定义文档查询；默认使用 docRepo.FindByID
	DocLookup func(docID int64) (*KnowledgeDocument, error)
}

// NewScheduleJob 构造。
func NewScheduleJob(repo ScheduleRepo, docRepo DocRepo, fetcher *HTTPFetcher, proc ScheduleDocProcessor, cfg ScheduleJobConfig) *ScheduleJob {
	if cfg.LockSeconds <= 0 {
		cfg.LockSeconds = 900
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 20
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = 10 * time.Second
	}
	j := &ScheduleJob{repo: repo, docRepo: docRepo, fetcher: fetcher, proc: proc, cfg: cfg}
	if docRepo != nil {
		j.DocLookup = func(docID int64) (*KnowledgeDocument, error) { return docRepo.FindByID(docID) }
	}
	return j
}

// Start 阻塞运行，直到 ctx 被取消。main.go 应该在独立 goroutine 调用。
func (j *ScheduleJob) Start(ctx context.Context) {
	ticker := time.NewTicker(j.cfg.ScanInterval)
	defer ticker.Stop()

	zap.L().Info("schedule job started",
		zap.String("owner", j.cfg.Owner),
		zap.Duration("interval", j.cfg.ScanInterval))

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("schedule job stopped")
			return
		case <-ticker.C:
			j.scan(ctx)
		}
	}
}

func (j *ScheduleJob) scan(ctx context.Context) {
	now := time.Now()
	due, err := j.repo.FindDue(now, j.cfg.BatchSize)
	if err != nil {
		zap.L().Error("find due schedules failed", zap.Error(err))
		return
	}

	lockUntil := time.Now().Add(time.Duration(j.cfg.LockSeconds) * time.Second)
	for _, s := range due {
		acquired, err := j.repo.TryAcquireLock(s.ID, j.cfg.Owner, lockUntil)
		if err != nil || !acquired {
			continue
		}
		go j.ProcessOne(ctx, s.ID)
	}
}

// ProcessOne 处理单条调度记录；测试可以直接调用。
func (j *ScheduleJob) ProcessOne(ctx context.Context, scheduleID int64) {
	defer func() { _ = j.repo.ReleaseLock(scheduleID, j.cfg.Owner) }()

	schedule, err := findScheduleByID(j.repo, scheduleID)
	if err != nil || schedule == nil {
		return
	}

	doc, err := j.DocLookup(schedule.DocID)
	if err != nil || doc == nil || doc.Enabled == 0 ||
		doc.SourceType != string(SourceTypeURL) || doc.ScheduleCron == "" {
		j.markSkipped(schedule, "文档不可用或不再需要调度", "")
		return
	}

	startTime := time.Now()
	exec := &KnowledgeDocumentScheduleExec{
		ScheduleID: scheduleID,
		DocID:      doc.ID,
		KbID:       doc.KbID,
		Status:     string(ScheduleRunning),
		StartTime:  startTime,
	}
	_ = j.repo.ExecCreate(exec)

	// 1. HEAD 快速判断
	head, headErr := j.fetcher.Head(doc.SourceLocation)
	if headErr == nil {
		// ETag 没变，说明内容没变，不需要处理了。
		if etag := head.ETag; etag != "" && etag == schedule.LastEtag {
			j.markSkipped(schedule, "ETag 未变化", etag)
			j.finishExec(exec, string(ScheduleSkipped), "ETag 未变化", startTime)
			j.advanceNextRun(schedule, doc.ScheduleCron)
			return
		}
		// Last-Modified 没变，说明内容没变，不需要处理了。
		if lm := head.LastModified; lm != "" && lm == schedule.LastModified {
			j.markSkipped(schedule, "Last-Modified 未变化", schedule.LastEtag)
			j.finishExec(exec, string(ScheduleSkipped), "Last-Modified 未变化", startTime)
			j.advanceNextRun(schedule, doc.ScheduleCron)
			return
		}
		// 大小超过限制，不处理了。
		if j.cfg.MaxFileBytes > 0 && head.ContentLength > j.cfg.MaxFileBytes {
			j.markFailed(schedule, "远程文件过大")
			j.finishExec(exec, string(ScheduleFailed), "远程文件过大", startTime)
			return
		}
	}

	// 2. 全量 GET + hash
	result, err := j.fetcher.Get(doc.SourceLocation, j.cfg.MaxFileBytes)
	if err != nil {
		j.markFailed(schedule, err.Error())
		j.finishExec(exec, string(ScheduleFailed), err.Error(), startTime)
		return
	}
	if result.ContentHash != "" && result.ContentHash == schedule.LastContentHash {
		j.markSkipped(schedule, "内容哈希未变化", result.ETag)
		j.finishExec(exec, string(ScheduleSkipped), "内容哈希未变化", startTime)
		j.advanceNextRun(schedule, doc.ScheduleCron)
		return
	}

	// 3. 交给下游处理（上传 S3 + 重分块）
	fileName := result.FileName
	if fileName == "" {
		fileName = doc.DocName
	}
	if err := j.proc.Process(ctx, doc.ID, result.Body, fileName, result.ContentType); err != nil {
		j.markFailed(schedule, err.Error())
		j.finishExec(exec, string(ScheduleFailed), err.Error(), startTime)
		return
	}

	// 4. 标记成功
	now := time.Now()
	schedule.LastEtag = result.ETag
	schedule.LastModified = result.LastModified
	schedule.LastContentHash = result.ContentHash
	schedule.LastSuccessTime = &now
	schedule.LastStatus = string(ScheduleSuccess)
	schedule.LastError = ""
	schedule.LastRunTime = &startTime
	j.advanceNextRun(schedule, doc.ScheduleCron)

	j.finishExec(exec, string(ScheduleSuccess), "刷新成功", startTime)
}

// 更新下一次运行时间，成功或失败都调用。
func (j *ScheduleJob) advanceNextRun(s *KnowledgeDocumentSchedule, cronExpr string) {
	next, err := NextRunTime(cronExpr, time.Now())
	if err == nil {
		s.NextRunTime = &next
	}
	_ = j.repo.UpdateAfterRun(s)
}

// 标记本次调度跳过，失败或文档不可用时调用。
func (j *ScheduleJob) markSkipped(s *KnowledgeDocumentSchedule, msg, etag string) {
	s.LastStatus = string(ScheduleSkipped)
	s.LastError = msg
	if etag != "" {
		s.LastEtag = etag
	}
	_ = j.repo.UpdateAfterRun(s)
}

func (j *ScheduleJob) markFailed(s *KnowledgeDocumentSchedule, errMsg string) {
	s.LastStatus = string(ScheduleFailed)
	s.LastError = truncate(errMsg, 512)
	_ = j.repo.UpdateAfterRun(s)
}

func (j *ScheduleJob) finishExec(exec *KnowledgeDocumentScheduleExec, status, message string, startTime time.Time) {
	end := time.Now()
	exec.Status = status
	exec.Message = truncate(message, 1024)
	exec.EndTime = &end
	_ = j.repo.ExecUpdate(exec)
}

func findScheduleByID(repo ScheduleRepo, scheduleID int64) (*KnowledgeDocumentSchedule, error) {
	return repo.FindByID(scheduleID)
}
