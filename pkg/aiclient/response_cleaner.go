package aiclient

import (
	"regexp"
	"strings"
)

var (
	leadingCodeFence  = regexp.MustCompile("^```[\\w-]*\\s*\\n?")
	trailingCodeFence = regexp.MustCompile("\\n?```\\s*$")
)

func StripMarkdownCodeFence(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = leadingCodeFence.ReplaceAllString(cleaned, "")
	cleaned = trailingCodeFence.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}
