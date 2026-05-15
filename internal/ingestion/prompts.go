// Package ingestion 提供文档摄入 pipeline。
//
// 本文件存放 ingestion 阶段 LLM 加工节点（Enhancer / Enricher）的 prompt 模板。
// 直接用 const 字符串而不是工厂类——Java 端的 EnhancerPromptManager 是 Spring
// 自动注入路线，Go 没有等价机制，硬模拟反而绕。
package ingestion

// enhancerSystemPrompt 指导 LLM 对整篇文档做结构化抽取。
//
// 要求返回 JSON 包含摘要和关键词；只返回 JSON 是为了让下游 json.Unmarshal 直接
// 解析，不必再写 markdown / 自然语言文本剥离逻辑。
const enhancerSystemPrompt = `你是文档分析助手。请阅读用户提供的文档全文，抽取以下信息并以 JSON 返回：
{
  "summary": "文档摘要，100 字以内",
  "keywords": ["关键词1", "关键词2", ...]
}
keywords 控制在 5-10 个。只返回 JSON，不要额外解释。`

// enricherSystemPrompt 指导 LLM 对单个 chunk 做加工。
//
// 生成"这段能回答哪些问题"是关键——这些 question 会被 EnricherNode 拼进
// EmbedText，让 chunk embedding 同时匹配"原文措辞"和"用户提问措辞"，
// 显著提升 RAG 召回率（HyDE / hypothetical-questions 模式）。
const enricherSystemPrompt = `你是文本块分析助手。请阅读用户提供的文本片段，生成以下信息并以 JSON 返回：
{
  "summary": "这段文字的摘要，50 字以内",
  "questions": ["这段能回答的问题1", "问题2", "问题3"]
}
questions 控制在 2-4 个。只返回 JSON，不要额外解释。`
