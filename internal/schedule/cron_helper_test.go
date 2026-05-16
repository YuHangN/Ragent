package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNextRunTime_EveryMinute(t *testing.T) {
	from := time.Date(2026, 4, 23, 10, 0, 30, 0, time.UTC)
	next, err := NextRunTime("0 * * * * *", from)
	assert.NoError(t, err)
	expected := time.Date(2026, 4, 23, 10, 1, 0, 0, time.UTC)
	assert.Equal(t, expected, next)
}

func TestNextRunTime_DailyAtMidnight(t *testing.T) {
	from := time.Date(2026, 4, 22, 15, 30, 0, 0, time.UTC)
	next, err := NextRunTime("0 0 0 * * *", from)
	assert.NoError(t, err)
	expected := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, next)
}

func TestNextRunTime_InvalidExpression(t *testing.T) {
	_, err := NextRunTime("not-a-cron", time.Now())
	assert.Error(t, err)
}

func TestNextRunTime_EmptyExpression(t *testing.T) {
	_, err := NextRunTime("", time.Now())
	assert.Error(t, err)
}
