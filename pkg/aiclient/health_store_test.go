package aiclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthStore_InitialState_AllowCall(t *testing.T) {
	hs := NewHealthStore(3, 100*time.Millisecond)
	assert.True(t, hs.AllowCall("model-a"))
}

func TestHealthStore_FailureThreshold_OpensCircuit(t *testing.T) {
	hs := NewHealthStore(3, 100*time.Millisecond)
	hs.MarkFailure("model-a")
	hs.MarkFailure("model-a")
	assert.True(t, hs.AllowCall("model-a")) // 2 次还没到阈值
	hs.MarkFailure("model-a")
	assert.False(t, hs.AllowCall("model-a")) // 3 次后熔断
}

func TestHealthStore_OpenToHalfOpen_AfterDuration(t *testing.T) {
	hs := NewHealthStore(1, 50*time.Millisecond)
	hs.MarkFailure("model-a") // 直接进 OPEN
	assert.False(t, hs.AllowCall("model-a"))

	time.Sleep(60 * time.Millisecond)
	// 第一次 AllowCall 进 HALF_OPEN，放行
	assert.True(t, hs.AllowCall("model-a"))
	// 第二次此时 HALF_OPEN inFlight，拒绝
	assert.False(t, hs.AllowCall("model-a"))
}

func TestHealthStore_HalfOpenSuccess_ClosesCircuit(t *testing.T) {
	hs := NewHealthStore(1, 50*time.Millisecond)
	hs.MarkFailure("model-a")
	time.Sleep(60 * time.Millisecond)
	assert.True(t, hs.AllowCall("model-a")) // HALF_OPEN
	hs.MarkSuccess("model-a")
	assert.True(t, hs.AllowCall("model-a")) // 已 CLOSED
}

func TestHealthStore_HalfOpenFailure_ReopensCircuit(t *testing.T) {
	hs := NewHealthStore(1, 50*time.Millisecond)
	hs.MarkFailure("model-a")
	time.Sleep(60 * time.Millisecond)
	assert.True(t, hs.AllowCall("model-a")) // HALF_OPEN
	hs.MarkFailure("model-a")
	assert.False(t, hs.AllowCall("model-a")) // 重新 OPEN
}

func TestHealthStore_MarkSuccess_ResetsCounter(t *testing.T) {
	hs := NewHealthStore(3, 100*time.Millisecond)
	hs.MarkFailure("model-a")
	hs.MarkFailure("model-a")
	hs.MarkSuccess("model-a") // 重置计数
	hs.MarkFailure("model-a")
	hs.MarkFailure("model-a")
	assert.True(t, hs.AllowCall("model-a")) // 仅 2 次失败，未达阈值
}
