package schedule

type RunStatus string

const (
	StatusRunning RunStatus = "running"
	StatusSuccess RunStatus = "success"
	StatusFailed  RunStatus = "failed"
	StatusSkipped RunStatus = "skipped"
)

func (s RunStatus) String() string { return string(s) }
