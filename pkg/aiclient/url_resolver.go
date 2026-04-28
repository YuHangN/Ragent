package aiclient

import (
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/config"
)

// ResolveURL 解析模型完整 URL。优先级：candidate.URL > provider.URL + endpoint。
// 对齐 Java ModelUrlResolver.resolveUrl。
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
