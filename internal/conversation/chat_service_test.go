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
// onRetrieve 由用例设置，可用于观察请求参数或返回固定结果和错误。
type mockRetriever struct {
	onRetrieve func(req retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error)
}

func (m *mockRetriever) Retrieve(_ context.Context, req retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
	return m.onRetrieve(req)
}

// mockLLM 是 aiclient.LLMService 的内存桩。
//
// Chat 和 StreamChat 行为分别由 onChat、onStream 注入；流式用例可通过回调模拟
// delta、完成和错误事件。
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

// captureCallback 收集 StreamMessage 的回调事件，供测试断言时序。
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

// captureRecorder 收集 ChatService 记录的 trace，供测试断言耗时与结果。
type captureRecorder struct {
	records []*admin.TraceRecord
}

func (c *captureRecorder) Record(t *admin.TraceRecord) {
	c.records = append(c.records, t)
}

// newTestChat 创建测试用的 ChatService 和底层 repo，便于断言落库状态。
func newTestChat(t *testing.T, rag *mockRetriever, llm *mockLLM) (*ChatService, *mockRepo) {
	t.Helper()
	repo := newMockRepo()
	conv := NewConversationService(repo)
	// recorder 为 nil 时会退化为空实现；这些用例不关心 trace 落库。
	return NewChatService(conv, rag, llm, nil), repo
}

// newTestChatWithRecorder 创建带 captureRecorder 的测试 ChatService。
func newTestChatWithRecorder(t *testing.T, rag *mockRetriever, llm *mockLLM) (*ChatService, *mockRepo, *captureRecorder) {
	t.Helper()
	repo := newMockRepo()
	conv := NewConversationService(repo)
	rec := &captureRecorder{}
	return NewChatService(conv, rag, llm, rec), repo, rec
}

// fixedChunks 返回用例共用的假召回片段。
func fixedChunks() []retrieval.RetrievedChunk {
	return []retrieval.RetrievedChunk{
		{ID: "c1", Content: "chunk one", Score: 0.9, KbID: 1},
		{ID: "c2", Content: "chunk two", Score: 0.7, KbID: 1},
	}
}

// fixedRetrieveResult 构造标准 RAG 返回值；messages 只需满足后续 LLM 调用即可。
func fixedRetrieveResult() *retrieval.RetrieveResult {
	return &retrieval.RetrieveResult{
		RewrittenQuery: "rewritten",
		SubQuestions:   []string{"rewritten"},
		Chunks:         fixedChunks(),
		Messages:       []aiclient.ChatMessage{aiclient.System("你是助手"), aiclient.User("rewritten")},
	}
}

// ──── SendMessage ────────────────────────────────────────

// TestSendMessage_HappyPath 验证同步问答端到端会落库 user 和 assistant 消息，
// 并返回 answer 与 chunks。
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

	// 成功路径应写入 user 和 assistant 两条消息。
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 2)
	assert.Equal(t, aiclient.RoleUser, msgs[0].Role)
	assert.Equal(t, "什么是 RAG？", msgs[0].Content)
	assert.Empty(t, msgs[0].ChunksJSON, "user 消息不应有 chunks_json")
	assert.Equal(t, aiclient.RoleAssistant, msgs[1].Role)
	assert.Equal(t, "RAG 是检索增强生成", msgs[1].Content)
	assert.Contains(t, msgs[1].ChunksJSON, `"c1"`, "assistant 消息应带 chunks_json")
}

// TestSendMessage_RetrieveFailure_KeepsUserSkipsAssistant 验证 RAG 失败时保留
// user 消息，但不写 assistant 消息。
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

// TestSendMessage_LLMFailure_KeepsUserSkipsAssistant 验证 LLM 失败时也只写 user 消息。
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
// 不含本次提问。
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

// TestStreamMessage_HappyPath_AccumulatesDeltas 验证流式正常路径会实时回调 delta，
// 并在流结束后完成落库和 OnComplete。
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

	// delta 顺序与发出顺序一致。
	assert.Equal(t, []string{"Hello", ", ", "world"}, cap.deltas)

	// OnComplete 被调用，answer 等于所有 delta 拼接。
	assert.True(t, cap.complete)
	assert.Equal(t, "Hello, world", cap.answer)
	assert.Len(t, cap.chunks, 2)
	assert.Empty(t, cap.errs)

	// assistant 消息一次性落库，content 与累积 answer 一致。
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 2)
	assert.Equal(t, aiclient.RoleAssistant, msgs[1].Role)
	assert.Equal(t, "Hello, world", msgs[1].Content)
	assert.Contains(t, msgs[1].ChunksJSON, `"c1"`)
}

// TestStreamMessage_LLMFailure_DoesNotPersistAssistant 验证流式 LLM 失败时不写
// assistant，并且不重复发送 OnError。
func TestStreamMessage_LLMFailure_DoesNotPersistAssistant(t *testing.T) {
	rr := &mockRetriever{onRetrieve: func(_ retrieval.RetrieveRequest) (*retrieval.RetrieveResult, error) {
		return fixedRetrieveResult(), nil
	}}
	streamErr := errors.New("stream cut")
	llm := &mockLLM{onStream: func(_ aiclient.ChatRequest, cb aiclient.StreamCallback) error {
		cb.OnContent("partial")
		cb.OnError(streamErr) // 模拟 LLMService 在返回错误前已通知回调。
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

	// OnError 应只发送一次。
	require.Len(t, cap.errs, 1, "OnError 不应被 StreamMessage 重复触发")
	assert.False(t, cap.complete, "失败路径不应发 OnComplete")

	// DB 中只保留 user 消息，不写 assistant。
	msgs := repo.msgs[conv.ID]
	require.Len(t, msgs, 1)
	assert.Equal(t, aiclient.RoleUser, msgs[0].Role)
}

// TestStreamMessage_RetrieveFailure_ReportsOnError 验证 RAG 失败时由 StreamMessage
// 主动发送 OnError。
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

// TestSendMessage_RecordsSuccessTrace 验证同步问答成功时会记录 Success=1 的 trace，
// 并填充用户、改写查询和 chunks 数等关键字段。
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

// TestSendMessage_RecordsFailureTrace 验证 RAG 失败时也会记录失败 trace。
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

// TestStreamMessage_RecordsSuccessTrace 验证流式问答成功时也会记录成功 trace。
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
