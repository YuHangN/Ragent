package aiclient

import "unicode"

type TokenCounter interface {
	Count(text string) int
}

type HeuristicTokenCounter struct{}

func NewHeuristicTokenCounter() *HeuristicTokenCounter {
	return &HeuristicTokenCounter{}
}

func (HeuristicTokenCounter) Count(text string) int {
	if text == "" {
		return 0
	}
	asciiCount, cjkCount, otherCount := 0, 0, 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		switch {
		case r <= 0x7F:
			asciiCount++
		case isCJK(r):
			cjkCount++
		default:
			otherCount++
		}
	}

	if asciiCount == 0 && cjkCount == 0 && otherCount == 0 {
		return 0 // 全是空白
	}

	asciiTokens := (asciiCount + 3) / 4
	otherTokens := (otherCount + 1) / 2
	total := asciiTokens + cjkCount + otherTokens

	if total < 1 {
		total = 1
	}
	return total
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // CJK 统一表意
		return true
	case r >= 0x3400 && r <= 0x4DBF: // 扩展 A
		return true
	case r >= 0x20000 && r <= 0x2A6DF: // 扩展 B
		return true
	case r >= 0x2A700 && r <= 0x2EBEF: // 扩展 C/D/E/F
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK 兼容
		return true
	case r >= 0x2F800 && r <= 0x2FA1F: // CJK 兼容补充
		return true
	case r >= 0x2E80 && r <= 0x2EFF: // 部首补充
		return true
	case r >= 0x3000 && r <= 0x303F: // CJK 符号和标点
		return true
	case r >= 0x3040 && r <= 0x309F: // 平假名
		return true
	case r >= 0x30A0 && r <= 0x30FF: // 片假名
		return true
	case r >= 0x31F0 && r <= 0x31FF: // 片假名语音扩展
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // 韩文音节
		return true
	case r >= 0x1100 && r <= 0x11FF: // 韩文 Jamo
		return true
	case r >= 0x3130 && r <= 0x318F: // 韩文兼容 Jamo
		return true
	}

	return false
}
