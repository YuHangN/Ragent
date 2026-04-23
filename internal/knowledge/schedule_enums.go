package knowledge

type ScheduleRunStatus string

const (
	ScheduleRunning ScheduleRunStatus = "running"
	ScheduleSuccess ScheduleRunStatus = "success"
	ScheduleFailed  ScheduleRunStatus = "failed"
	ScheduleSkipped ScheduleRunStatus = "skipped"
)

func (s ScheduleRunStatus) String() string { return string(s) }
