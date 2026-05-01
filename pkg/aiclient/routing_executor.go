package aiclient

import (
	"fmt"
	"reflect"
)

func ExecuteWithFallback[C any, T any](
	hs *HealthStore,
	capability Capability,
	targets []ModelTarget,
	resolveClient func(*ModelTarget) C,
	call func(C, *ModelTarget) (T, error),
) (T, error) {
	var zero T
	if len(targets) == 0 {
		return zero, fmt.Errorf("no %s candidates available", capability.DisplayName())
	}

	var lastErr error
	for i := range targets {
		target := &targets[i]
		client := resolveClient(target)

		// nil client 表示这个 provider 没注册，跳过
		if isNilInterface(client) {
			continue
		}

		// 熔断检查
		if !hs.AllowCall(target.ID) {
			continue
		}

		result, err := call(client, target)
		if err != nil {
			lastErr = err
			hs.MarkFailure(target.ID)
			continue
		}

		hs.MarkSuccess(target.ID)
		return result, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no available %s candidate (all skipped)", capability.DisplayName())
	}

	return zero, fmt.Errorf("all %s candidates failed: %w", capability.DisplayName(), lastErr)
}

// isNilInterface 检查泛型参数实例是否为 nil interface。
func isNilInterface(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return rv.IsNil()
	}
	return false
}
