package parser

import (
	"regexp"
	"strings"
)

var (
	// multiBlankLine 用于将连续 3 个及以上换行压缩为 2 个换行。
	//
	// 保留一个空行可以维持段落边界，同时避免解析结果中出现大段无意义空白，
	// 降低后续切分、索引和检索阶段的噪声。
	multiBlankLine = regexp.MustCompile(`\n{3,}`)

	// zeroWidth 匹配常见零宽字符。
	//
	// 这些字符通常来自网页、富文本或复制粘贴场景，对展示不可见，但会影响
	// 文本匹配、去重、分词和向量化结果，因此在入库前统一移除。
	zeroWidth = regexp.MustCompile(`[\x{200B}-\x{200D}\x{FEFF}]`)
)

// CleanupText 对解析得到的原始文本进行基础清洗和规范化。
//
// 清洗规则保持克制，只处理跨解析器普遍存在的问题：
//  1. 将 Windows/macOS 旧式换行统一为 Unix 换行；
//  2. 移除零宽字符，避免不可见字符污染检索语料；
//  3. 将过多连续空行压缩为单个段落间隔；
//  4. 去除首尾空白，保证返回内容可直接进入后续切分流程。
//
// 该函数不会改变正文内部的普通空格和单换行，避免破坏代码块、表格或
// 其他对空白敏感的文本结构。
func CleanupText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = zeroWidth.ReplaceAllString(s, "")
	s = multiBlankLine.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
