package knowledge

import (
	"strings"
	"time"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/robfig/cron/v3"
)

// 字段顺序：秒 分 时 日 月 周
var cronParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// NextRunTime 解析 cron 表达式并返回从 from 开始的下次触发时间。
func NextRunTime(expr string, from time.Time) (time.Time, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return time.Time{}, apperror.NewClientMsg("cron 表达式不能为空")
	}

	schedule, err := cronParser.Parse(expr)
	if err != nil {
		return time.Time{}, apperror.NewClientMsg("cron 表达式不合法：" + expr)
	}

	return schedule.Next(from), nil
}

// IsIntervalLessThan 判断 cron 两次触发之间的间隔是否短于 minSeconds。
func IsIntervalLessThan(expr string, from time.Time, minSeconds int64) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true
	}
	schedule, err := cronParser.Parse(expr)
	if err != nil {
		return true
	}
	first := schedule.Next(from)
	second := schedule.Next(first)
	if first.IsZero() || second.IsZero() {
		return true
	}
	diff := second.Sub(first).Seconds()
	return int64(diff) < minSeconds
}
