package intent

import (
	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// Classifier 使用 LLM 给意图节点打分。
//
// 它不负责拆分问题，也不负责决定后续走哪个检索通道。它只做一件事：
// 给某个 KB 下"可分类的意图节点"计算相关性分数，并转成 Candidate。
//
// 同一个 Classifier 实例对外暴露两套打分入口：
//   - ClassifyQuery       —— 对单个用户问题打分，见 classify_query.go
//   - ClassifyChunks —— 对批量文档片段打分，见 classify_chunk.go
//
// 两者共用同一组 LLM + intent 仓库，只是 prompt 模板与输入语义不同。
type Classifier struct {
	llm        aiclient.LLMService
	intentRepo Repo
}

// NewClassifier 创建意图分类器。
//
// llm 用来判断"问题/片段和意图节点有多相关"；repo 用来加载指定 KB 下可分类的意图节点。
func NewClassifier(llm aiclient.LLMService, repo Repo) *Classifier {
	return &Classifier{llm: llm, intentRepo: repo}
}
