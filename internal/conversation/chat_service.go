// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 本文件提供 ChatService，把"会话历史 + RAG 检索 + LLM 调用"三者串成一条链路。
// 它通过两个最小接口分别接 RAG 与 LLM，而不是直接引用具体类型，方便单元测试时
// 把外部副作用替换成 mock。
package conversation

import (
	"context"
	"encoding/json"

	"github.com/YuHangN/ragent-go/internal/rag"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// historyMaxMessages 控制每次 chat 时从持久化历史中加载的最大消息数。
//
// MVP 写死 20 条，约对应 10 轮问答。后续 Phase 7.5 ConversationSummaryService
// 会用 LLM 摘要替换远古消息，这里再改成"摘要 + 最近 N 条"模式。
const historyMaxMessages = 20

// ragRetriever 是 ChatService 实际需要的 RAG 能力子集。
//
// 显式定义接口而不是直接依赖 *rag.RAGCoreService，是为了让单测能注入一个 mock
// 实现，避免在 chat 测试里启 Milvus + 真 LLM。
// 生产环境直接传 *rag.RAGCoreService 即可（它的 Retrieve 方法签名兼容）。
type ragRetriever interface {
	Retrieve(ctx context.Context, req rag.RetrieveRequest) (*rag.RetrieveResult, error)
}

// ChatService 串联会话历史、RAG 检索与 LLM 调用。
//
// 它本身不持有任何全局状态，所有持久化操作委托给 ConversationService，
// RAG 与 LLM 通过注入的接口调用。这种依赖结构让测试可以独立验证业务规则，
// 不需要真正的 DB / 向量库 / 模型。
type ChatService struct {
	conv *ConversationService
	rag  ragRetriever
	llm  aiclient.LLMService
}

// NewChatService 构造 ChatService。
//
// 生产路径在 main.go 里传入 *rag.RAGCoreService 和 routingLLMService；
// 单元测试里传入 mock 实现即可。
func NewChatService(conv *ConversationService, ragSvc ragRetriever, llm aiclient.LLMService) *ChatService {
	return &ChatService{conv: conv, rag: ragSvc, llm: llm}
}

// SendRequest 是 SendMessage / StreamMessage 共用入参。
//
// TopK 为 0 时由 RAGCoreService 内部兜底（默认 5）。KbIDs 为空表示纯系统问答，
// RAG 链路会走 SYSTEM 短路或全局兜底，由 Phase 6 内部决定。
type SendRequest struct {
	ConversationID int64
	Question       string
	KbIDs          []int64
	TopK           int
}

// SendResponse 是同步问答的返回结果。
//
// Chunks 携带本次 RAG 召回的元信息，便于前端展开引用、做 citation 跳转。
type SendResponse struct {
	Answer string
	Chunks []rag.RetrievedChunk
}

// SendMessage 执行同步问答链路。
//
// 步骤：
//  1. LoadHistory 拉取最近 20 条上下文（不含本次问题）
//  2. AppendMessage 持久化 user 消息
//  3. ragCore.Retrieve 跑改写→意图→多通道检索→拼 prompt
//  4. llm.Chat 阻塞拿完整答案
//  5. AppendMessage 持久化 assistant 消息（携带 chunks 元信息）
//
// 失败策略：第 3、4 步任意一步出错都直接返回，user 消息保留在 DB（"用户问过
// 这一句的事实"应当存档），但不写 assistant 消息——半成品答案会污染未来历史。
// 调用方拿到 err 自行决定前端如何提示。
func (s *ChatService) SendMessage(ctx context.Context, req SendRequest) (*SendResponse, error) {
	history, err := s.conv.LoadHistory(req.ConversationID, historyMaxMessages)
	if err != nil {
		return nil, err
	}

	if _, err := s.conv.AppendMessage(req.ConversationID, aiclient.RoleUser, req.Question, ""); err != nil {
		return nil, err
	}

	retrieved, err := s.rag.Retrieve(ctx, rag.RetrieveRequest{
		KbIDs:    req.KbIDs,
		Question: req.Question,
		History:  history,
		TopK:     req.TopK,
	})
	if err != nil {
		return nil, err
	}

	answer, err := s.llm.Chat(ctx, aiclient.ChatRequest{Messages: retrieved.Messages})
	if err != nil {
		return nil, err
	}

	// chunks 序列化失败时只丢空串，不阻塞答案返回。
	chunksJSON, _ := json.Marshal(retrieved.Chunks)
	if _, err := s.conv.AppendMessage(req.ConversationID, aiclient.RoleAssistant, answer, string(chunksJSON)); err != nil {
		return nil, err
	}

	return &SendResponse{Answer: answer, Chunks: retrieved.Chunks}, nil
}
