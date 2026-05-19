// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 本文件提供 ChatService，把"会话历史 + RAG 检索 + LLM 调用"三者串成一条链路。
// 它通过两个最小接口分别接 RAG 与 LLM，而不是直接引用具体类型，方便单元测试时
// 把外部副作用替换成 mock。每次问答还会把耗时与结果汇总成一条 trace 记录。
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

// historyMaxMessages 控制每次 chat 时从持久化历史中加载的最大消息数。
//
// MVP 写死 20 条，约对应 10 轮问答。后续 Phase 7.5 ConversationSummaryService
// 会用 LLM 摘要替换远古消息，这里再改成"摘要 + 最近 N 条"模式。
const historyMaxMessages = 20

// ragRetriever 是 ChatService 实际需要的 RAG 能力子集。
//
// 显式定义接口而不是直接依赖 *retrieval.RAGCoreService，是为了让单测能注入一个 mock
// 实现，避免在 chat 测试里启 Milvus + 真 LLM。
// 生产环境直接传 *retrieval.RAGCoreService 即可（它的 Retrieve 方法签名兼容）。
type ragRetriever interface {
	Retrieve(ctx context.Context, req retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error)
}

// ChatService 串联会话历史、RAG 检索与 LLM 调用。
//
// 它本身不持有任何全局状态，所有持久化操作委托给 ConversationService，
// RAG 与 LLM 通过注入的接口调用。recorder 用于把每次问答的耗时/结果落 trace，
// 走 noopRecorder 时等价于不记。
type ChatService struct {
	conv     *ConversationService
	rag      ragRetriever
	llm      aiclient.LLMService
	recorder admin.TraceRecorder
}

// NewChatService 构造 ChatService。
//
// 生产路径在 main.go 里传入 *retrieval.RAGCoreService 和 routingLLMService；
// recorder 传 nil 时退化为 noopRecorder，调用方主路径不必判空。
func NewChatService(conv *ConversationService, ragSvc ragRetriever, llm aiclient.LLMService, recorder admin.TraceRecorder) *ChatService {
	if recorder == nil {
		recorder = admin.NewNoopRecorder()
	}
	return &ChatService{conv: conv, rag: ragSvc, llm: llm, recorder: recorder}
}

// SendRequest 是 SendMessage / StreamMessage 共用入参。
//
// TopK 为 0 时由 RAGCoreService 内部兜底（默认 5）。KbIDs 为空表示纯系统问答，
// RAG 链路会走 SYSTEM 短路或全局兜底，由 Phase 6 内部决定。
// UserID 仅用于 trace 归属，不参与业务逻辑。
type SendRequest struct {
	ConversationID int64
	UserID         int64
	Question       string
	KbIDs          []int64
	TopK           int
}

// SendResponse 是同步问答的返回结果。
//
// Chunks 携带本次 RAG 召回的元信息，便于前端展开引用、做 citation 跳转。
type SendResponse struct {
	Answer string
	Chunks []retrieval.RetrievedChunk
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
//
// 无论成功失败，defer 都会落一条 trace：阶段耗时按实际到达的步骤填，未到达的
// 阶段保持 0。
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

	// chunks 序列化失败时只丢空串，不阻塞答案返回。
	chunksJSON, _ := json.Marshal(retrieved.Chunks)
	if _, err := s.conv.AppendMessage(req.ConversationID, aiclient.RoleAssistant, answer, string(chunksJSON)); err != nil {
		tr.markError(err)
		return nil, err
	}

	tr.markSuccess()
	return &SendResponse{Answer: answer, Chunks: retrieved.Chunks}, nil
}

// StreamCallback 是 chat 流式回调，由 ChatService.StreamMessage 调用。
//
// 与 aiclient.StreamCallback 的区别：这里只保留业务层关心的事件（增量内容、
// 结束、错误），不暴露 thinking 流；OnComplete 携带完整答案与召回的 chunk，
// 便于 HTTP 层一次性发"done"事件。
type StreamCallback interface {
	OnDelta(delta string)
	OnComplete(answer string, chunks []retrieval.RetrievedChunk)
	OnError(err error)
}

// StreamMessage 执行流式问答链路。
//
// 前置步骤（拉历史、写 user、跑 RAG）与 SendMessage 完全一致；区别只在 LLM
// 调用走 StreamChat，并把每个 delta 通过回调实时推给 HTTP 层。完整答案在
// 流自然结束时一次性持久化——避免每 token 一次 DB write 的写放大。
//
// 失败策略与 SendMessage 一致：任何前置步骤或 LLM 失败都不写 assistant 消息，
// 保留干净的对话历史；err 已经通过 cb.OnError 通知给 HTTP 层，函数同时返回
// 该 err 让调用方可以记日志。
//
// trace：defer 落一条记录，与 SendMessage 同样的阶段计时口径。
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
		// aiclient.LLMService.StreamChat 内部已调用 bridge.OnError；这里只需把
		// 错误向上抛，不重复发 cb.OnError，避免前端收到两条 error event。
		tr.markError(err)
		return err
	}

	// 流自然结束：把累积答案落库 + 发 done 事件给前端。
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

// streamBridge 把 aiclient.StreamCallback（底层 LLM 流式接口）适配成
// ChatService.StreamCallback（业务层接口）。
//
// 同时累积 OnContent delta 到 strings.Builder，供流结束后一次性落库——避免
// 每 token 一次 DB write 的写放大。OnComplete 故意为空，由上层 StreamMessage
// 在确认所有副作用都成功后才发业务层 OnComplete，保证"持久化失败就不发 done"。
type streamBridge struct {
	cb     StreamCallback
	answer strings.Builder
}

func (b *streamBridge) OnContent(delta string) {
	b.answer.WriteString(delta)
	b.cb.OnDelta(delta)
}

// OnThinking 在 MVP 阶段忽略——业务层 StreamCallback 不暴露 thinking 流。
// 后续如要把思维链展示给前端，再扩 StreamCallback 增加 OnThinkingDelta 即可。
func (b *streamBridge) OnThinking(_ string) {}

// OnComplete 故意空实现：上层 StreamMessage 在持久化 assistant 消息成功后
// 才发业务层 cb.OnComplete，保证 done 事件与 DB 落盘同步。
func (b *streamBridge) OnComplete() {}

func (b *streamBridge) OnError(err error) {
	b.cb.OnError(err)
}

// ──── trace 累加器 ───────────────────────────────────────

// traceBuilder 是 SendMessage / StreamMessage 共用的 trace 累加器。
//
// 它在请求开始时记录起点，过程中分段填阶段耗时与结果字段，最后由 build()
// 算出总耗时并产出 *admin.TraceRecord。设计成"逐步填充"是为了配合 defer：
// 无论链路在哪一步失败，defer 都能拿到当前已填的部分。
type traceBuilder struct {
	startTime time.Time
	record    *admin.TraceRecord
	historyMs int64
	ragMs     int64
	llmMs     int64
}

// newTraceBuilder 用请求入参初始化累加器。Success 默认 0（失败），只有显式
// markSuccess 才置 1——这样任何中途 return 都自然记为失败。
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

// fillFromRetrieve 把 RAG 检索结果的关键字段抄进 trace。
func (b *traceBuilder) fillFromRetrieve(r *retrieval.RetrieveResult) {
	b.record.RewrittenQuery = r.RewrittenQuery
	if data, err := json.Marshal(r.SubQuestions); err == nil {
		b.record.SubQuestionsJSON = string(data)
	}
	b.record.ChunksCount = len(r.Chunks)
}

// markError 记录失败原因；Success 保持 0。
func (b *traceBuilder) markError(err error) {
	b.record.ErrorMessage = err.Error()
}

// markSuccess 把 trace 标记为成功。
func (b *traceBuilder) markSuccess() {
	b.record.Success = 1
}

// build 填入阶段耗时与总耗时，产出最终 trace 记录。
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
