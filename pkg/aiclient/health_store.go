package aiclient

import (
	"sync"
	"time"
)

type healthState int

const (
	// stateClosed 表示模型健康，调用可正常放行。
	stateClosed healthState = iota
	// stateOpen 表示模型处于熔断期，调用会被跳过。
	stateOpen
	// stateHalfOpen 表示熔断期结束后的探测状态，只允许一个探测请求。
	stateHalfOpen
)

// modelHealth 记录单个模型目标的熔断状态。
type modelHealth struct {
	state               healthState
	consecutiveFailures int
	openUntil           time.Time
	halfOpenInFlight    bool
}

// HealthStore 保存模型目标的熔断状态，并负责调用放行判断。
type HealthStore struct {
	mu               sync.Mutex
	healthByID       map[string]*modelHealth
	failureThreshold int
	openDuration     time.Duration
}

// NewHealthStore 构造熔断状态存储。
//
// failureThreshold 和 openDuration 小于等于 0 时分别回退到 3 次失败和 30 秒。
func NewHealthStore(failureThreshold int, openDuration time.Duration) *HealthStore {
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	if openDuration <= 0 {
		openDuration = 30 * time.Second
	}
	return &HealthStore{
		healthByID:       map[string]*modelHealth{},
		failureThreshold: failureThreshold,
		openDuration:     openDuration,
	}
}

// AllowCall 判断指定模型当前是否允许发起调用。
//
// CLOSED 直接放行；OPEN 在熔断期结束后进入 HALF_OPEN 并放行一次探测；
// HALF_OPEN 只允许一个正在进行的探测请求。
func (hs *HealthStore) AllowCall(id string) bool {
	if id == "" {
		return false
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	h := hs.healthByID[id]
	if h == nil {
		h = &modelHealth{state: stateClosed}
		hs.healthByID[id] = h
	}

	now := time.Now()
	switch h.state {
	case stateClosed:
		return true
	case stateOpen:
		if now.After(h.openUntil) {
			h.state = stateHalfOpen
			h.halfOpenInFlight = true
			return true
		}
		return false
	case stateHalfOpen:
		if h.halfOpenInFlight {
			return false
		}
		h.halfOpenInFlight = true
		return true
	}
	return false
}

// MarkSuccess 标记指定模型调用成功，并将熔断状态重置为 CLOSED。
func (hs *HealthStore) MarkSuccess(id string) {
	if id == "" {
		return
	}
	hs.mu.Lock()
	defer hs.mu.Unlock()

	h := hs.healthByID[id]
	if h == nil {
		hs.healthByID[id] = &modelHealth{state: stateClosed}
		return
	}
	h.state = stateClosed
	h.consecutiveFailures = 0
	h.openUntil = time.Time{}
	h.halfOpenInFlight = false
}

// MarkFailure 标记指定模型调用失败，并在达到阈值时打开熔断。
//
// HALF_OPEN 下的失败会立即重新进入 OPEN；CLOSED 下的失败会累计计数。
func (hs *HealthStore) MarkFailure(id string) {
	if id == "" {
		return
	}
	hs.mu.Lock()
	defer hs.mu.Unlock()

	h := hs.healthByID[id]
	if h == nil {
		h = &modelHealth{state: stateClosed}
		hs.healthByID[id] = h
	}
	now := time.Now()

	if h.state == stateHalfOpen {
		h.state = stateOpen
		h.openUntil = now.Add(hs.openDuration)
		h.consecutiveFailures = 0
		h.halfOpenInFlight = false
		return
	}

	h.consecutiveFailures++
	if h.consecutiveFailures >= hs.failureThreshold {
		h.state = stateOpen
		h.openUntil = now.Add(hs.openDuration)
		h.consecutiveFailures = 0
		h.halfOpenInFlight = false
	}
}

// IsOpen 判断指定模型是否仍处于 OPEN 熔断期。
//
// 该方法只用于 Selector 提前过滤，不会消耗 HALF_OPEN 的探测配额。
func (hs *HealthStore) IsOpen(id string) bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	h := hs.healthByID[id]
	if h == nil {
		return false
	}
	return h.state == stateOpen && time.Now().Before(h.openUntil)
}
