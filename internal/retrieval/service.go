package retrieval

import (
	"context"

	"github.com/YuHangN/ragent-go/config"
	"github.com/YuHangN/ragent-go/internal/intent"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// RAGCoreService 是 RAG 检索核心的统一入口。
type RAGCoreService struct {
	rewriter *QueryRewriteService
	resolver *intent.Resolver
	engine   *MultiChannelEngine
	prompt   *PromptService
	config   config.RAGConfig
}

func NewRAGCoreService(
	rewriter *QueryRewriteService,
	resolver *intent.Resolver,
	engine *MultiChannelEngine,
	prompt *PromptService,
	cfg config.RAGConfig,
) *RAGCoreService {
	return &RAGCoreService{
		rewriter: rewriter,
		resolver: resolver,
		engine:   engine,
		prompt:   prompt,
		config:   cfg,
	}
}

// Retrieve 执行完整的 RAG 检索链路，返回可直接传给 LLM 的结果。
// 链路：查询改写 → 意图分类（多子问题并行）→ 分组 → 多通道检索 → 去重重排 → Prompt 构建
func (s *RAGCoreService) Retrieve(ctx context.Context, req RetrieveRequest) (*RetrieveResult, error) {
	if req.TopK <= 0 {
		req.TopK = 5
	}

	// 1. 查询改写（失败时降级，不中断）
	rewriteResult, _ := s.rewriter.Rewrite(ctx, req.Question, req.History)
	if rewriteResult.RewrittenQuery == "" {
		rewriteResult.RewrittenQuery = req.Question
		rewriteResult.SubQuestions = []string{req.Question}
	}

	// 2. 意图解析（多子问题并行 → 分组）
	// subs 保留子问题→候选意图的绑定关系，供 channel 做 per-sub-question 路由；
	// group 是扁平合并视图，仅用于 SYSTEM 短路判断（findings.md Phase 6.6）。
	var subs []intent.SubQuestionIntent
	var group intent.Group
	if s.resolver != nil && len(req.KbIDs) > 0 {
		var err error
		subs, err = s.resolver.Resolve(ctx, req.KbIDs[0], rewriteResult.SubQuestions)
		if err == nil {
			group = s.resolver.MergeGroup(subs)
		}
	}

	// 2.5 SYSTEM 短路：仅当所有子问题都"全部命中 SYSTEM"时跳过检索。
	//     混合场景（如 "你好，介绍一下产品"）AllSystemOnly=false，继续走 KB 检索；
	//     "你好" 部分由 LLM 用 system prompt 自然应答，不需要特殊路径。
	if group.AllSystemOnly {
		return &RetrieveResult{
			RewrittenQuery: rewriteResult.RewrittenQuery,
			SubQuestions:   rewriteResult.SubQuestions,
			Chunks:         nil,
			Messages: []aiclient.ChatMessage{
				aiclient.System("你是一个友好的 AI 助手。"),
				aiclient.User(req.Question),
			},
		}, nil
	}

	// 3. 构建检索上下文，执行多通道检索
	sc := SearchContext{
		KbIDs:        req.KbIDs,
		Question:     rewriteResult.RewrittenQuery,
		SubQuestions: rewriteResult.SubQuestions,
		SubIntents:   subs,
		IntentGroup:  group,
		TopK:         req.TopK,
	}
	chunks, err := s.engine.Retrieve(ctx, sc)
	if err != nil {
		return nil, err
	}

	// 4. 构建 Prompt 消息序列（KB_ONLY 场景；MCP/MIXED 等到 Phase 10）
	messages := s.prompt.BuildMessages(PromptContext{
		Scene:    PromptSceneKBOnly,
		Question: req.Question,
		Chunks:   chunks,
		History:  req.History,
	})

	return &RetrieveResult{
		RewrittenQuery: rewriteResult.RewrittenQuery,
		SubQuestions:   rewriteResult.SubQuestions,
		Chunks:         chunks,
		Messages:       messages,
	}, nil
}
