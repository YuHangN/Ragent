package aiclient

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeClient struct {
	name string
}

func TestExecuteWithFallback_FirstSucceeds(t *testing.T) {
	hs := NewHealthStore(3, time.Second)
	targets := []ModelTarget{{ID: "a"}, {ID: "b"}}

	calls := 0
	result, err := ExecuteWithFallback(hs, CapabilityChat, targets,
		func(t *ModelTarget) *fakeClient { return &fakeClient{name: t.ID} },
		func(c *fakeClient, t *ModelTarget) (string, error) {
			calls++
			return "ok-" + c.name, nil
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, "ok-a", result)
	assert.Equal(t, 1, calls, "第一个成功后不应调用第二个")
}

func TestExecuteWithFallback_FirstFailsSecondSucceeds(t *testing.T) {
	hs := NewHealthStore(3, time.Second)
	targets := []ModelTarget{{ID: "a"}, {ID: "b"}}

	result, err := ExecuteWithFallback(
		hs, CapabilityChat, targets,
		func(t *ModelTarget) *fakeClient { return &fakeClient{name: t.ID} },
		func(c *fakeClient, t *ModelTarget) (string, error) {
			if c.name == "a" {
				return "", errors.New("a failed")
			}
			return "ok-" + c.name, nil
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, "ok-b", result)
}

func TestExecuteWithFallback_AllFail(t *testing.T) {
	hs := NewHealthStore(3, time.Second)
	targets := []ModelTarget{{ID: "a"}, {ID: "b"}}

	_, err := ExecuteWithFallback(
		hs, CapabilityChat, targets,
		func(t *ModelTarget) *fakeClient { return &fakeClient{name: t.ID} },
		func(c *fakeClient, t *ModelTarget) (string, error) {
			return "", errors.New(c.name + " down")
		},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all Chat candidates failed")
}

func TestExecuteWithFallback_NoTargets(t *testing.T) {
	hs := NewHealthStore(3, time.Second)
	_, err := ExecuteWithFallback(
		hs, CapabilityChat, nil,
		func(t *ModelTarget) *fakeClient { return nil },
		func(c *fakeClient, t *ModelTarget) (string, error) { return "", nil },
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Chat candidates")
}

func TestExecuteWithFallback_ClientNil_Skipped(t *testing.T) {
	hs := NewHealthStore(3, time.Second)
	targets := []ModelTarget{{ID: "a"}, {ID: "b"}}

	calls := 0
	result, err := ExecuteWithFallback(
		hs, CapabilityChat, targets,
		func(t *ModelTarget) *fakeClient {
			if t.ID == "a" {
				return nil // a 没注册 client
			}
			return &fakeClient{name: t.ID}
		},
		func(c *fakeClient, t *ModelTarget) (string, error) {
			calls++
			return "ok-" + c.name, nil
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, "ok-b", result)
	assert.Equal(t, 1, calls)
}

func TestExecuteWithFallback_FailureUpdatesHealthStore(t *testing.T) {
	hs := NewHealthStore(1, 1*time.Hour) // 阈值 1
	targets := []ModelTarget{{ID: "a"}, {ID: "b"}}

	_, _ = ExecuteWithFallback(
		hs, CapabilityChat, targets,
		func(t *ModelTarget) *fakeClient { return &fakeClient{name: t.ID} },
		func(c *fakeClient, t *ModelTarget) (string, error) {
			if c.name == "a" {
				return "", errors.New("a failed")
			}
			return "ok", nil
		},
	)
	assert.True(t, hs.IsOpen("a"), "失败的应被熔断")
	assert.False(t, hs.IsOpen("b"), "成功的不应熔断")
}
