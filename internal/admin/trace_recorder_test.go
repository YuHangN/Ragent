package admin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRepo 是 recorder 测试使用的内存 TraceRepo。
//
// insertErr 不为 nil 时，Insert 返回该错误，用于验证 recorder 的错误隔离行为。
type stubRepo struct {
	inserted  []TraceRecord
	insertErr error
}

func (s *stubRepo) Insert(t *TraceRecord) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.inserted = append(s.inserted, *t)
	return nil
}

func (s *stubRepo) List(_, _ int) ([]TraceRecord, int64, error) {
	return s.inserted, int64(len(s.inserted)), nil
}

func (s *stubRepo) FindByID(_ int64) (*TraceRecord, error) {
	return nil, nil
}

// TestNoopRecorder_DoesNothing 验证空实现可调用且无副作用。
//
// trace 关闭时，调用 Record 应该既不 panic 也不落库。
func TestNoopRecorder_DoesNothing(t *testing.T) {
	r := NewNoopRecorder()
	assert.NotPanics(t, func() {
		r.Record(&TraceRecord{Question: "test"})
	})
}

// TestMySQLRecorder_PersistsRecord 验证同步落库路径。
//
// Record 应将传入的 TraceRecord 原样交给 repo.Insert。
func TestMySQLRecorder_PersistsRecord(t *testing.T) {
	repo := &stubRepo{}
	r := NewMySQLRecorder(repo, nil)

	r.Record(&TraceRecord{
		ConversationID: 100,
		UserID:         42,
		Question:       "什么是 RAG？",
		TotalMs:        150,
		Success:        1,
	})

	require.Len(t, repo.inserted, 1)
	assert.Equal(t, int64(100), repo.inserted[0].ConversationID)
	assert.Equal(t, int64(42), repo.inserted[0].UserID)
	assert.Equal(t, int64(150), repo.inserted[0].TotalMs)
}

// TestMySQLRecorder_SwallowsInsertError 验证落库失败不会 panic、不会向上抛错。
//
// trace 是旁路观测，落库失败不能阻断 chat 主路径。
func TestMySQLRecorder_SwallowsInsertError(t *testing.T) {
	repo := &stubRepo{insertErr: errors.New("db connection lost")}
	r := NewMySQLRecorder(repo, nil)

	assert.NotPanics(t, func() {
		r.Record(&TraceRecord{ConversationID: 1})
	})
	assert.Empty(t, repo.inserted, "落库失败时不应有记录被存入")
}

// TestNewMySQLRecorder_NilLoggerDoesNotPanic 验证 nil logger 会退化为 nop，
// 避免 Record 内部解引用 nil logger。
func TestNewMySQLRecorder_NilLoggerDoesNotPanic(t *testing.T) {
	repo := &stubRepo{insertErr: errors.New("forced error to trigger logger")}
	r := NewMySQLRecorder(repo, nil)

	assert.NotPanics(t, func() {
		r.Record(&TraceRecord{ConversationID: 1})
	})
}
