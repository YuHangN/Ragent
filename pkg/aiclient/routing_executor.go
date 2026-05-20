package aiclient

import (
	"fmt"
	"reflect"
)

// ExecuteWithFallback 按候选顺序执行调用，并在失败时切换到下一个可用目标。
//
// 该函数同时负责跳过未注册 client、更新熔断状态，以及在所有候选失败后返回
// 带有能力名称的聚合错误。
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

		// nil client 表示该 provider 没有注册对应能力的客户端。
		if isNilInterface(client) {
			continue
		}

		// 熔断中的目标直接跳过，让后续候选接管。
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

// isNilInterface 判断泛型参数实际承载的值是否为 nil。
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
