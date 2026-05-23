// Package conversation 提供 RAG Chat 的会话管理与问答链路。
//
// 本文件提供 ChatService，将会话历史、RAG 检索和 LLM 调用串成一次问答流程。
// RAG 与 LLM 通过最小接口注入，便于测试替换外部依赖；每次问答都会汇总为一条
// trace 记录。
package conversation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/YuHangN/ragent-go/internal/admin"
	"github.com/YuHangN/ragent-go/internal/retrieval"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// historyMaxMessages 是每次 chat 从持久化历史中加载的最大消息数。
//
// 当前只加载最近消息，避免长会话无限增长上下文。
const historyMaxMessages = 20

// ragRetriever 是 ChatService 需要的 RAG 能力子集。
//
// 显式定义接口可让单元测试注入 mock，避免启动真实检索依赖；生产环境可直接传入
// 兼容该方法签名的 RAG 实现。
type ragRetriever interface {
	Retrieve(ctx context.Context, req retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error)
}

// ChatService 串联会话历史、RAG 检索与 LLM 调用。
//
// 持久化操作委托给 ConversationService，RAG 与 LLM 通过注入接口调用。recorder
// 用于记录每次问答的耗时和结果。
type ChatService struct {
	conv     *ConversationService
	rag      ragRetriever
	llm      aiclient.LLMService
	recorder admin.TraceRecorder
}

// NewChatService 创建 ChatService。
//
// recorder 为 nil 时使用空实现，调用方主路径无需判空。
func NewChatService(conv *ConversationService, ragSvc ragRetriever, llm aiclient.LLMService, recorder admin.TraceRecorder) *ChatService {
	if recorder == nil {
		recorder = admin.NewNoopRecorder()
	}
	return &ChatService{conv: conv, rag: ragSvc, llm: llm, recorder: recorder}
}

// SendRequest 是 SendMessage 和 StreamMessage 共用的请求参数。
//
// TopK 为 0 时由检索链路使用默认值；KbIDs 为空表示不在请求层限定知识库范围。
// UserID 仅用于 trace 归属，不参与问答业务逻辑。
type SendRequest struct {
	ConversationID int64
	UserID         int64
	Question       string
	KbIDs          []int64
	TopK           int
}

// SendResponse 是同步问答的返回结果。
//
// Chunks 携带本次 RAG 召回的元信息，供前端展示引用。
type SendResponse struct {
	Answer string
	Chunks []retrieval.RetrievedChunk
}

// SendMessage 执行同步问答链路。
//
// 步骤：
//  1. LoadHistory 拉取历史上下文，不含本次问题
//  2. AppendMessage 持久化 user 消息
//  3. RAG Retrieve 生成 LLM 所需消息和召回片段
//  4. LLM Chat 阻塞获取完整答案
//  5. AppendMessage 持久化 assistant 消息，并写入 chunks 元信息
//
// 失败策略：RAG 或 LLM 失败时保留 user 消息，但不写 assistant 消息，避免半成品
// 答案进入后续历史。
//
// 无论成功失败，defer 都会记录 trace；未到达的阶段耗时保持 0。
func (s *ChatService) SendMessage(ctx context.Context, req SendRequest) (*SendResponse, error) {
	tr := newTraceBuilder(req)
	defer func() { s.recorder.Record(tr.build()) }()

	histStart := time.Now()
	history, err := s.conv.LoadHistory(req.ConversationID, historyMaxMessages)
	tr.historyMs = msSince(histStart)
	if err != nil {
		tr.markError(err)
		return nil, err
	}

	if _, err := s.conv.AppendMessage(req.ConversationID, aiclient.RoleUser, req.Question, ""); err != nil {
		tr.markError(err)
		return nil, err
	}

	ragStart := time.Now()
	retrieved, err := s.rag.Retrieve(ctx, retrieval.RetrieveRequest{
		KbIDs:    req.KbIDs,
		Question: req.Question,
		History:  history,
		TopK:     req.TopK,
	})
	tr.ragMs = msSince(ragStart)
	if err != nil {
		tr.markError(err)
		return nil, err
	}
	tr.fillFromRetrieve(retrieved)

	llmStart := time.Now()
	answer, err := s.llm.Chat(ctx, aiclient.ChatRequest{Messages: retrieved.Messages})
	tr.llmMs = msSince(llmStart)
	if err != nil {
		tr.markError(err)
		return nil, err
	}

	// chunks 序列化失败时写空串，不阻塞答案返回。
	chunksJSON, _ := json.Marshal(retrieved.Chunks)
	if _, err := s.conv.AppendMessage(req.ConversationID, aiclient.RoleAssistant, answer, string(chunksJSON)); err != nil {
		tr.markError(err)
		return nil, err
	}

	tr.markSuccess()
	return &SendResponse{Answer: answer, Chunks: retrieved.Chunks}, nil
}

// StreamCallback 是 ChatService.StreamMessage 使用的业务层流式回调。
//
// 它只暴露业务层需要的增量内容、完成和错误事件；OnComplete 携带完整答案与
// 召回片段，便于 HTTP 层发送最终事件。
type StreamCallback interface {
	OnDelta(delta string)
	OnComplete(answer string, chunks []retrieval.RetrievedChunk)
	OnError(err error)
}

// StreamMessage 执行流式问答链路。
//
// 拉历史、写 user 和 RAG 检索与 SendMessage 保持一致；LLM 调用改用 StreamChat，
// 并把 delta 通过回调实时推给上层。完整答案在流结束后一次性持久化，避免每个
// token 都写数据库。
//
// 失败策略与 SendMessage 一致：任何前置步骤或 LLM 失败都不写 assistant 消息。
// 错误会通过回调通知上层，同时作为返回值交给调用方记录日志。
//
// trace 记录与 SendMessage 使用相同的阶段计时口径。
func (s *ChatService) StreamMessage(ctx context.Context, req SendRequest, cb StreamCallback) error {
	tr := newTraceBuilder(req)
	defer func() { s.recorder.Record(tr.build()) }()

	histStart := time.Now()
	history, err := s.conv.LoadHistory(req.ConversationID, historyMaxMessages)
	tr.historyMs = msSince(histStart)
	if err != nil {
		tr.markError(err)
		cb.OnError(err)
		return err
	}

	if _, err := s.conv.AppendMessage(req.ConversationID, aiclient.RoleUser, req.Question, ""); err != nil {
		tr.markError(err)
		cb.OnError(err)
		return err
	}

	ragStart := time.Now()
	retrieved, err := s.rag.Retrieve(ctx, retrieval.RetrieveRequest{
		KbIDs:    req.KbIDs,
		Question: req.Question,
		History:  history,
		TopK:     req.TopK,
	})
	tr.ragMs = msSince(ragStart)
	if err != nil {
		tr.markError(err)
		cb.OnError(err)
		return err
	}
	tr.fillFromRetrieve(retrieved)

	bridge := &streamBridge{cb: cb}
	llmStart := time.Now()
	err = s.llm.StreamChat(ctx, aiclient.ChatRequest{Messages: retrieved.Messages}, bridge)
	tr.llmMs = msSince(llmStart)
	if err != nil {
		// StreamChat 内部已调用 bridge.OnError；这里仅返回错误，避免重复发送
		// error 事件。
		tr.markError(err)
		return err
	}

	// 流自然结束后，先持久化累积答案，再发送完成事件。
	chunksJSON, _ := json.Marshal(retrieved.Chunks)
	answer := bridge.answer.String()
	if _, err := s.conv.AppendMessage(req.ConversationID, aiclient.RoleAssistant, answer, string(chunksJSON)); err != nil {
		tr.markError(err)
		cb.OnError(err)
		return err
	}

	tr.markSuccess()
	cb.OnComplete(answer, retrieved.Chunks)
	return nil
}

// streamBridge 将底层 LLM 流式回调适配为业务层流式回调。
//
// 它会累积所有 OnContent delta，供流结束后一次性落库。OnComplete 为空实现，
// 由 StreamMessage 在持久化成功后再触发业务层完成事件。
type streamBridge struct {
	cb     StreamCallback
	answer strings.Builder
}

func (b *streamBridge) OnContent(delta string) {
	b.answer.WriteString(delta)
	b.cb.OnDelta(delta)
}

// OnThinking 当前不转发，业务层 StreamCallback 不暴露 thinking 流。
func (b *streamBridge) OnThinking(_ string) {}

// OnComplete 为空实现；StreamMessage 会在 assistant 消息持久化成功后发送完成事件。
func (b *streamBridge) OnComplete() {}

func (b *streamBridge) OnError(err error) {
	b.cb.OnError(err)
}

// ──── trace 累加器 ───────────────────────────────────────

// traceBuilder 是 SendMessage 和 StreamMessage 共用的 trace 累加器。
//
// 它在请求开始时记录起点，过程中逐步填充阶段耗时与结果字段，最后由 build
// 计算总耗时并产出 TraceRecord。该结构便于 defer 在任意失败点记录已完成部分。
type traceBuilder struct {
	startTime time.Time
	record    *admin.TraceRecord
	historyMs int64
	ragMs     int64
	llmMs     int64
}

// newTraceBuilder 使用请求参数初始化累加器。Success 默认保持 0，只有显式
// markSuccess 后才置为 1。
func newTraceBuilder(req SendRequest) *traceBuilder {
	return &traceBuilder{
		startTime: time.Now(),
		record: &admin.TraceRecord{
			ConversationID: req.ConversationID,
			UserID:         req.UserID,
			Question:       req.Question,
		},
	}
}

// fillFromRetrieve 将 RAG 检索结果的关键字段写入 trace。
func (b *traceBuilder) fillFromRetrieve(r *retrieval.RetrieveResult) {
	b.record.RewrittenQuery = r.RewrittenQuery
	if data, err := json.Marshal(r.SubQuestions); err == nil {
		b.record.SubQuestionsJSON = string(data)
	}
	b.record.ChunksCount = len(r.Chunks)
}

// markError 记录失败原因，Success 保持 0。
func (b *traceBuilder) markError(err error) {
	b.record.ErrorMessage = err.Error()
}

// markSuccess 把 trace 标记为成功。
func (b *traceBuilder) markSuccess() {
	b.record.Success = 1
}

// build 填入阶段耗时与总耗时，并返回最终 trace 记录。
func (b *traceBuilder) build() *admin.TraceRecord {
	b.record.HistoryMs = b.historyMs
	b.record.RagMs = b.ragMs
	b.record.LLMMs = b.llmMs
	b.record.TotalMs = msSince(b.startTime)
	return b.record
}

// msSince 返回从 t 到现在的毫秒数。
func msSince(t time.Time) int64 {
	return time.Since(t).Milliseconds()
}
