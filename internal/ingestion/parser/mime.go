package parser

import (
	"bytes"
	"path/filepath"
	"strings"
)

// DefaultMimeType 是无法识别文件类型时使用的兜底 MIME 类型。
//
// 使用二进制流作为默认值可以避免调用方误判为文本内容，并符合通用
// HTTP/文件上传场景中对未知内容的处理方式。
const DefaultMimeType = "application/octet-stream"

// DetectMimeType 根据文件名和文件内容识别 MIME 类型。
//
// 识别策略按“确定性优先”执行：
//  1. 优先使用文件扩展名识别，便于区分 Office Open XML 等共享 ZIP 容器的格式；
//  2. 扩展名不可用或不受支持时，回退到文件头魔数识别；
//  3. 仍无法识别时，返回 DefaultMimeType。
//
// 该函数不校验文件内容与扩展名是否一致，调用方如需安全校验应在更高层
// 对 MIME 类型、文件签名和业务白名单进行组合验证。
func DetectMimeType(data []byte, fileName string) string {
	if mime := detectByExt(fileName); mime != "" {
		return mime
	}
	if mime := detectByMagic(data); mime != "" {
		return mime
	}
	return DefaultMimeType
}

// detectByExt 根据文件扩展名识别 MIME 类型。
//
// 扩展名识别具备更强的格式区分能力，例如 .docx、.xlsx、.pptx 底层
// 都是 ZIP 容器，仅依赖魔数无法准确区分具体文档类型。
func detectByExt(fileName string) string {
	if fileName == "" {
		return ""
	}

	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	}

	return ""
}

// detectByMagic 根据文件头魔数识别 MIME 类型。
//
// 魔数识别仅作为扩展名识别失败后的补充手段，覆盖常见文档和结构化文本
// 场景。对于 ZIP 容器类格式，该函数只返回通用 application/zip，不尝试
// 解析容器内部结构。
func detectByMagic(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return "application/pdf"
	case bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0}):
		return "application/msword" // 老 Office 格式 (CFB)
	case bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x03, 0x04}):
		return "application/zip" // .docx/.xlsx/.pptx 都是 zip；调用方靠扩展名细分
	case bytes.HasPrefix(data, []byte("<!DOCTYPE html")) ||
		bytes.HasPrefix(data, []byte("<html")):
		return "text/html"
	case bytes.HasPrefix(data, []byte("{")) || bytes.HasPrefix(data, []byte("[")):
		return "application/json"
	}

	return ""
}
