package aiclient

import (
	"sync"
	"time"
)

type healthState int

const (
	stateClosed healthState = iota
	stateOpen
	stateHalfOpen
)

// modelHealth 单个模型的健康记录。
type modelHealth struct {
	state               healthState
	consecutiveFailures int
	openUntil           time.Time
	halfOpenInFlight    bool
}

// HealthStore 熔断器存储
type HealthStore struct {
	mu               sync.Mutex
	healthByID       map[string]*modelHealth
	failureThreshold int
	openDuration     time.Duration
}

// NewHealthStore 构造熔断器。failureThreshold=3 / openDuration=30s 是 Java 默认值。
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

// AllowCall 该模型当前是否允许调用。
// CLOSED → 允许
// OPEN：openUntil 已过 → 升级到 HALF_OPEN 并允许；否则拒绝
// HALF_OPEN：inFlight=true 拒绝（已有 probe 在路上）；否则 inFlight=true 并允许
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

// MarkSuccess 调用成功。重置状态到 CLOSED + 清零计数。
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

// MarkFailure 调用失败。
// HALF_OPEN：直接重开 OPEN（probe 失败说明 provider 还没好）。
// CLOSED：累计 +1，达到阈值时进 OPEN。
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

// IsOpen 仅供 Selector 提前过滤用，不消耗 HALF_OPEN 配额。
func (hs *HealthStore) IsOpen(id string) bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	h := hs.healthByID[id]
	if h == nil {
		return false
	}
	return h.state == stateOpen && time.Now().Before(h.openUntil)
}
