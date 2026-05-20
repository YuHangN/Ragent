package aiclient

import (
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/config"
)

// ResolveURL 解析指定能力的完整调用 URL。
//
// 解析优先级为 candidate.URL > provider.URL + provider.Endpoints[capability]。
func ResolveURL(provider config.ProviderConfig, candidate config.ModelCandidate, capability Capability) (string, error) {
	if candidate.URL != "" {
		return candidate.URL, nil
	}
	if provider.URL == "" {
		return "", fmt.Errorf("provider baseUrl is missing for capability=%s", capability)
	}
	key := strings.ToLower(string(capability))
	path, ok := provider.Endpoints[key]
	if !ok || path == "" {
		return "", fmt.Errorf("provider endpoint is missing: %s", key)
	}
	return joinURL(provider.URL, path), nil
}

// joinURL 拼接 base URL 和 endpoint path，并处理边界斜杠。
func joinURL(base, path string) string {
	switch {
	case strings.HasSuffix(base, "/") && strings.HasPrefix(path, "/"):
		return base + path[1:]
	case !strings.HasSuffix(base, "/") && !strings.HasPrefix(path, "/"):
		return base + "/" + path
	default:
		return base + path
	}
}
