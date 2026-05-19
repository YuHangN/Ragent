package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/YuHangN/ragent-go/internal/admin"
	"github.com/YuHangN/ragent-go/internal/retrieval"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──── 测试桩 ──────────────────────────────────────────────

// mockRetriever 是 ragRetriever 接口的内存桩。
//
// onRetrieve 由用例设置，可以在调用时观察入参（验证 history / question 传递）或
// 返回固定结果 / 错误。无副作用，每个用例新建一个实例。
type mockRetriever struct {
	onRetrieve func(req retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error)
}

func (m *mockRetriever) Retrieve(_ context.Context, req retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
	return m.onRetrieve(req)
}

// mockLLM 是 aiclient.LLMService 的内存桩。
//
// Chat / StreamChat 行为分别由 onChat / onStream 注入。StreamChat 桩里可以通过
// cb 调用 OnContent / OnComplete / OnError 来模拟流事件序列。
type mockLLM struct {
	onChat   func(req aiclient.ChatRequest) (string, error)
	onStream func(req aiclient.ChatRequest, cb aiclient.StreamCallback) error
}

func (m *mockLLM) Chat(_ context.Context, req aiclient.ChatRequest) (string, error) {
	return m.onChat(req)
}

func (m *mockLLM) StreamChat(_ context.Context, req aiclient.ChatRequest, cb aiclient.StreamCallback) error {
	return m.onStream(req, cb)
}

// captureCallback 收集 ChatService.StreamMessage 的回调事件，供测试断言时序。
type captureCallback struct {
	deltas   []string
	complete bool
	answer   string
	chunks   []retrieval.RetrievedChunk
	errs     []error
}

func (c *captureCallback) OnDelta(delta string) {
	c.deltas = append(c.deltas, delta)
}
func (c *captureCallback) OnComplete(answer string, chunks []retrieval.RetrievedChunk) {
	c.complete = true
	c.answer = answer
	c.chunks = chunks
}
func (c *captureCallback) OnError(err error) {
	c.errs = append(c.errs, err)
}

// captureRecorder 收集 ChatService 落下的每条 trace，供测试断言耗时与结果。
type captureRecorder struct {
	records []*admin.TraceRecord
}

func (c *captureRecorder) Record(t *admin.TraceRecord) {
	c.records = append(c.records, t)
}

// newTestChat 构造一组测试用的 ChatService + 底层 repo，方便用例直接断言落库状态。
func newTestChat(t *testing.T, rag *mockRetriever, llm *mockLLM) (*ChatService, *mockRepo) {
	t.Helper()
	repo := newMockRepo()
	conv := NewConversationService(repo)
	// recorder 传 nil → NewChatService 内部退化为 noopRecorder，测试不关心 trace 落库。
	return NewChatService(conv, rag, llm, nil), repo
}

// newTestChatWithRecorder 与 newTestChat 相同，但额外注入一个 captureRecorder，
// 供需要断言 trace 内容的用例使用。
func newTestChatWithRecorder(t *testing.T, rag *mockRetriever, llm *mockLLM) (*ChatService, *mockRepo, *captureRecorder) {
	t.Helper()
	repo := newMockRepo()
	conv := NewConversationService(repo)
	rec := &captureRecorder{}
	return NewChatService(conv, rag, llm, rec), repo, rec
}

// fixedChunks 是用例共用的两条假 chunk，模拟 RAG 召回。
func fixedChunks() []retrieval.RetrievedChunk {
	return []retrieval.RetrievedChunk{
		{ID: "c1", Content: "chunk one", Score: 0.9, KbID: 1},
		{ID: "c2", Content: "chunk two", Score: 0.7, KbID: 1},
	}
}

// fixedRetrieveResult 构造一个标准的 RAG 返回值，messages 是任意非空切片即可，
// chat_service 只把它原样喂给 LLM 桩，不解析内容。
func fixedRetrieveResult() *retrieval.RetrieveResult {
	return &retrieval.RetrieveResult{
		RewrittenQuery: "rewritten",
		SubQuestions:   []string{"rewritten"},
		Chunks:         fixedChunks(),
		Messages:       []aiclient.ChatMessage{aiclient.System("你是助手"), aiclient.User("rewritten")},
	}
}

// ──── SendMessage ────────────────────────────────────────

// TestSendMessage_HappyPath 验证同步问答端到端：user 与 assistant 消息都落库，
// 返回结构带正确的 answer + chunks。
func TestSendMessage_HappyPath(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return fixedRetrieveResult(), nil
	}}
	llm := &mockLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return "RAG 是检索增强生成", nil
	}}

	svc, repo := newTestChat(t, rr, llm)
	conv, _ := svc.conv.CreateSession(42, []int64{1}, "")

	resp, err := svc.SendMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "什么是 RAG？",
		KbIDs:          []int64{1},
		TopK:           5,
	})

	require.NoError(t, err)
	assert.Equal(t, "RAG 是检索增强生成", resp.Answer)
	assert.Len(t, resp.Chunks, 2)

	// 应该两条消息：user + assistant
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 2)
	assert.Equal(t, aiclient.RoleUser, msgs[0].Role)
	assert.Equal(t, "什么是 RAG？", msgs[0].Content)
	assert.Empty(t, msgs[0].ChunksJSON, "user 消息不应有 chunks_json")
	assert.Equal(t, aiclient.RoleAssistant, msgs[1].Role)
	assert.Equal(t, "RAG 是检索增强生成", msgs[1].Content)
	assert.Contains(t, msgs[1].ChunksJSON, `"c1"`, "assistant 消息应带 chunks_json")
}

// TestSendMessage_RetrieveFailure_KeepsUserSkipsAssistant 验证 RAG 失败时
// user 消息保留、assistant 不落库——半成品答案不能污染未来历史。
func TestSendMessage_RetrieveFailure_KeepsUserSkipsAssistant(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return nil, errors.New("milvus down")
	}}
	llm := &mockLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		t.Fatal("LLM 不该被调用")
		return "", nil
	}}

	svc, repo := newTestChat(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")

	_, err := svc.SendMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "Q",
	})

	require.Error(t, err)
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 1, "只应落 user 消息")
	assert.Equal(t, aiclient.RoleUser, msgs[0].Role)
}

// TestSendMessage_LLMFailure_KeepsUserSkipsAssistant 验证 LLM 失败时也只落 user 消息。
func TestSendMessage_LLMFailure_KeepsUserSkipsAssistant(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return fixedRetrieveResult(), nil
	}}
	llm := &mockLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return "", errors.New("openai 429")
	}}

	svc, repo := newTestChat(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")

	_, err := svc.SendMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "Q",
	})

	require.Error(t, err)
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 1, "LLM 失败也只该有 user 消息")
}

// TestSendMessage_PassesHistoryWithoutCurrentQuestion 验证传给 RAG 的 history
// 不含本次提问——LoadHistory 必须在 AppendMessage(user) 之前调用，否则本次问题
// 会被回放给 RAG 当作"先前历史"。
func TestSendMessage_PassesHistoryWithoutCurrentQuestion(t *testing.T) {
	var capturedHistory []aiclient.ChatMessage
	rr := &mockRetriever{onRetrieve: func(req retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		capturedHistory = req.History
		return fixedRetrieveResult(), nil
	}}
	llm := &mockLLM{onChat: func(_ aiclient.ChatRequest) (string, error) { return "A2", nil }}

	svc, _ := newTestChat(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")
	_, _ = svc.conv.AppendMessage(conv.ID, aiclient.RoleUser, "Q1", "")
	_, _ = svc.conv.AppendMessage(conv.ID, aiclient.RoleAssistant, "A1", "")

	_, err := svc.SendMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "Q2",
	})
	require.NoError(t, err)

	require.Len(t, capturedHistory, 2, "history 应只含 Q1 / A1，不含本次 Q2")
	assert.Equal(t, "Q1", capturedHistory[0].Content)
	assert.Equal(t, "A1", capturedHistory[1].Content)
}

// ──── StreamMessage ───────────────────────────────────────

// TestStreamMessage_HappyPath_AccumulatesDeltas 验证流式正常路径：
// delta 实时回调 + 流结束后才发 OnComplete + assistant 消息一次性落库。
func TestStreamMessage_HappyPath_AccumulatesDeltas(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return fixedRetrieveResult(), nil
	}}
	llm := &mockLLM{onStream: func(_ aiclient.ChatRequest, cb aiclient.StreamCallback) error {
		cb.OnContent("Hello")
		cb.OnContent(", ")
		cb.OnContent("world")
		return nil
	}}

	svc, repo := newTestChat(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")
	cap := &captureCallback{}

	err := svc.StreamMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "say hi",
	}, cap)
	require.NoError(t, err)

	// delta 顺序与发出顺序一致
	assert.Equal(t, []string{"Hello", ", ", "world"}, cap.deltas)

	// OnComplete 被调用，answer 等于所有 delta 拼接
	assert.True(t, cap.complete)
	assert.Equal(t, "Hello, world", cap.answer)
	assert.Len(t, cap.chunks, 2)
	assert.Empty(t, cap.errs)

	// assistant 消息一次性落库，content 与累积 answer 一致
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 2)
	assert.Equal(t, aiclient.RoleAssistant, msgs[1].Role)
	assert.Equal(t, "Hello, world", msgs[1].Content)
	assert.Contains(t, msgs[1].ChunksJSON, `"c1"`)
}

// TestStreamMessage_LLMFailure_DoesNotPersistAssistant 验证流式 LLM 失败时
// 不写 assistant，避免半成品答案污染历史；OnError 由 LLMService 内部已发，
// StreamMessage 不再重复发。
func TestStreamMessage_LLMFailure_DoesNotPersistAssistant(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return fixedRetrieveResult(), nil
	}}
	streamErr := errors.New("stream cut")
	llm := &mockLLM{onStream: func(_ aiclient.ChatRequest, cb aiclient.StreamCallback) error {
		cb.OnContent("partial")
		cb.OnError(streamErr) // 模拟 aiclient.LLMService 失败前已通知 cb
		return streamErr
	}}

	svc, repo := newTestChat(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")
	cap := &captureCallback{}

	err := svc.StreamMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "Q",
	}, cap)
	require.ErrorIs(t, err, streamErr)

	// OnError 应只被发一次（由 LLM 桩内部发，StreamMessage 不重复发）
	require.Len(t, cap.errs, 1, "OnError 不应被 StreamMessage 重复触发")
	assert.False(t, cap.complete, "失败路径不应发 OnComplete")

	// DB 里只剩 user 消息，assistant 没落
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 1)
	assert.Equal(t, aiclient.RoleUser, msgs[0].Role)
}

// TestStreamMessage_RetrieveFailure_ReportsOnError 验证 RAG 失败时 OnError
// 由 StreamMessage 主动发——区别于 LLM 失败的"已发不重发"。
func TestStreamMessage_RetrieveFailure_ReportsOnError(t *testing.T) {
	retrieveErr := errors.New("milvus down")
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return nil, retrieveErr
	}}
	llm := &mockLLM{onStream: func(_ aiclient.ChatRequest, _ aiclient.StreamCallback) error {
		t.Fatal("LLM 不该被调用")
		return nil
	}}

	svc, _ := newTestChat(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")
	cap := &captureCallback{}

	err := svc.StreamMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "Q",
	}, cap)
	require.ErrorIs(t, err, retrieveErr)

	require.Len(t, cap.errs, 1)
	assert.ErrorIs(t, cap.errs[0], retrieveErr)
	assert.False(t, cap.complete)
}

// ──── trace 记录 ─────────────────────────────────────────

// TestSendMessage_RecordsSuccessTrace 验证同步问答成功时落一条 Success=1 的 trace，
// 且关键字段（用户、改写后 query、chunks 数）都被填上。
func TestSendMessage_RecordsSuccessTrace(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return fixedRetrieveResult(), nil
	}}
	llm := &mockLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		return "答案", nil
	}}

	svc, _, rec := newTestChatWithRecorder(t, rr, llm)
	conv, _ := svc.conv.CreateSession(7, []int64{1}, "")

	_, err := svc.SendMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		UserID:         7,
		Question:       "什么是 RAG？",
	})
	require.NoError(t, err)

	require.Len(t, rec.records, 1, "成功路径应落且只落一条 trace")
	tr := rec.records[0]
	assert.Equal(t, 1, tr.Success)
	assert.Empty(t, tr.ErrorMessage)
	assert.Equal(t, int64(7), tr.UserID)
	assert.Equal(t, conv.ID, tr.ConversationID)
	assert.Equal(t, "什么是 RAG？", tr.Question)
	assert.Equal(t, "rewritten", tr.RewrittenQuery)
	assert.Equal(t, 2, tr.ChunksCount)
	assert.GreaterOrEqual(t, tr.TotalMs, int64(0))
}

// TestSendMessage_RecordsFailureTrace 验证 RAG 失败时也落 trace，Success=0、
// ErrorMessage 非空——失败请求同样要被观测到。
func TestSendMessage_RecordsFailureTrace(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return nil, errors.New("milvus down")
	}}
	llm := &mockLLM{onChat: func(_ aiclient.ChatRequest) (string, error) {
		t.Fatal("LLM 不该被调用")
		return "", nil
	}}

	svc, _, rec := newTestChatWithRecorder(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")

	_, err := svc.SendMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "Q",
	})
	require.Error(t, err)

	require.Len(t, rec.records, 1, "失败路径同样要落 trace")
	tr := rec.records[0]
	assert.Equal(t, 0, tr.Success)
	assert.Contains(t, tr.ErrorMessage, "milvus down")
	assert.Zero(t, tr.LLMMs, "RAG 失败时 LLM 阶段未到达，耗时应为 0")
}

// TestStreamMessage_RecordsSuccessTrace 验证流式问答成功时也落一条 Success=1 的 trace。
func TestStreamMessage_RecordsSuccessTrace(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return fixedRetrieveResult(), nil
	}}
	llm := &mockLLM{onStream: func(_ aiclient.ChatRequest, cb aiclient.StreamCallback) error {
		cb.OnContent("a")
		cb.OnContent("b")
		return nil
	}}

	svc, _, rec := newTestChatWithRecorder(t, rr, llm)
	conv, _ := svc.conv.CreateSession(1, nil, "")

	err := svc.StreamMessage(context.Background(), SendRequest{
		ConversationID: conv.ID,
		Question:       "Q",
	}, &captureCallback{})
	require.NoError(t, err)

	require.Len(t, rec.records, 1)
	assert.Equal(t, 1, rec.records[0].Success)
	assert.Equal(t, 2, rec.records[0].ChunksCount)
}
