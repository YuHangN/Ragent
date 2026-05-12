package conversation

import (
	"errors"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepo 是测试用的内存版 ConversationRepo。
//
// 用 map 模拟存储；ID 由 nextID 分配，模拟 BeforeCreate 钩子。所有方法都不返回
// gorm 真实错误，只在"找不到会话"这种语义错时返回 sentinel，让用例可断言。
type mockRepo struct {
	convs map[int64]*Conversation
	msgs  map[int64][]Message
	seq   int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		convs: map[int64]*Conversation{},
		msgs:  map[int64][]Message{},
	}
}

func (m *mockRepo) nextID() int64 {
	m.seq++
	return m.seq
}

var errMockNotFound = errors.New("not found")

func (m *mockRepo) CreateConversation(c *Conversation) error {
	c.ID = m.nextID()
	m.convs[c.ID] = c
	return nil
}

func (m *mockRepo) UpdateConversation(c *Conversation) error {
	if _, ok := m.convs[c.ID]; !ok {
		return errMockNotFound
	}
	m.convs[c.ID] = c
	return nil
}

func (m *mockRepo) DeleteConversation(id int64) error {
	delete(m.convs, id)
	return nil
}

func (m *mockRepo) FindConversationByID(id int64) (*Conversation, error) {
	c, ok := m.convs[id]
	if !ok {
		return nil, errMockNotFound
	}
	return c, nil
}

func (m *mockRepo) ListConversationsByUser(userID int64, limit, offset int) ([]Conversation, int64, error) {
	var out []Conversation
	for _, c := range m.convs {
		if c.UserID == userID {
			out = append(out, *c)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockRepo) AppendMessage(msg *Message) error {
	msg.ID = m.nextID()
	m.msgs[msg.ConversationID] = append(m.msgs[msg.ConversationID], *msg)
	return nil
}

func (m *mockRepo) ListMessages(cid int64, limit int) ([]Message, error) {
	return m.msgs[cid], nil
}

// ──── 测试 ───────────────────────────────────────────────

func TestCreateSession_StoresUserAndKbIDs(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	conv, err := svc.CreateSession(42, []int64{1, 2, 3}, "")
	require.NoError(t, err)
	assert.Equal(t, int64(42), conv.UserID)
	assert.Empty(t, conv.Title) // 显式不传 title 时为空
	assert.Equal(t, "[1,2,3]", conv.KbIDs)
	assert.NotZero(t, conv.ID)
}

func TestAppendMessage_AutoFillsTitleFromFirstUserMessage(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	conv, _ := svc.CreateSession(1, nil, "")
	_, err := svc.AppendMessage(conv.ID, aiclient.RoleUser, "RAG 是什么？", "")
	require.NoError(t, err)

	updated, _ := svc.repo.FindConversationByID(conv.ID)
	assert.Equal(t, "RAG 是什么？", updated.Title)
}

func TestAppendMessage_DoesNotOverwriteExistingTitle(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	conv, _ := svc.CreateSession(1, nil, "已有标题")
	_, _ = svc.AppendMessage(conv.ID, aiclient.RoleUser, "新问题", "")

	updated, _ := svc.repo.FindConversationByID(conv.ID)
	assert.Equal(t, "已有标题", updated.Title, "已有 title 不应被覆盖")
}

func TestAppendMessage_AssistantRoleDoesNotTriggerTitleFill(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	conv, _ := svc.CreateSession(1, nil, "")
	_, _ = svc.AppendMessage(conv.ID, aiclient.RoleAssistant, "我先来一句", "")

	updated, _ := svc.repo.FindConversationByID(conv.ID)
	assert.Empty(t, updated.Title, "assistant 消息不应触发 title 自动填充")
}

func TestAppendMessage_ConversationNotFound(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	_, err := svc.AppendMessage(999, aiclient.RoleUser, "q", "")
	require.Error(t, err)
}

func TestLoadHistory_ReturnsTimeOrderedChatMessages(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	conv, _ := svc.CreateSession(1, nil, "")
	_, _ = svc.AppendMessage(conv.ID, aiclient.RoleUser, "Q1", "")
	_, _ = svc.AppendMessage(conv.ID, aiclient.RoleAssistant, "A1", `[{"id":"c1"}]`)
	_, _ = svc.AppendMessage(conv.ID, aiclient.RoleUser, "Q2", "")

	hist, err := svc.LoadHistory(conv.ID, 10)
	require.NoError(t, err)
	require.Len(t, hist, 3)
	assert.Equal(t, aiclient.RoleUser, hist[0].Role)
	assert.Equal(t, "Q1", hist[0].Content)
	assert.Equal(t, aiclient.RoleAssistant, hist[1].Role)
	assert.Equal(t, "A1", hist[1].Content)
	assert.Equal(t, "Q2", hist[2].Content)
}

func TestRenameTitle_OverwritesAnyExisting(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	conv, _ := svc.CreateSession(1, nil, "旧标题")
	require.NoError(t, svc.RenameTitle(conv.ID, "新标题"))

	updated, _ := svc.repo.FindConversationByID(conv.ID)
	assert.Equal(t, "新标题", updated.Title)
}

func TestRenameTitle_ConversationNotFound(t *testing.T) {
	svc := NewConversationService(newMockRepo())

	err := svc.RenameTitle(999, "anything")
	require.Error(t, err)
}

// ──── truncateTitle 行为 ─────────────────────────────────

func TestTruncateTitle_NoTruncationUnderLimit(t *testing.T) {
	assert.Equal(t, "short", truncateTitle("short", 30))
}

func TestTruncateTitle_TruncatesAtRuneBoundary(t *testing.T) {
	// 12 个中文字符，截到 5 应该是"测试一二三…"，不会截在 UTF-8 字节中间
	got := truncateTitle("测试一二三四五六七八九十", 5)
	assert.Equal(t, "测试一二三…", got)
}

func TestTruncateTitle_TrimsSurroundingWhitespace(t *testing.T) {
	assert.Equal(t, "hello", truncateTitle("  hello  ", 30))
}
