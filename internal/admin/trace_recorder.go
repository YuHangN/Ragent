// Package admin 提供 RAG 系统的运维接口与链路追踪能力。
//
// 本文件定义 ChatService 使用的链路追踪入口。业务主路径只依赖 Record 方法，
// 不需要关心 trace 是落库还是被忽略。
package admin

import "go.uber.org/zap"

// TraceRecorder 是 ChatService 调用的链路追踪接口。
//
// 设计要点：
//   - Record 不返回 error，trace 失败不能阻断 chat 主路径
//   - 通过不同实现区分关闭 trace 和写入 MySQL 的场景
type TraceRecorder interface {
	Record(t *TraceRecord)
}

// noopRecorder 是 trace 关闭时使用的空实现。
//
// 调用方始终注入非 nil recorder，避免在 chat 主路径散落判空逻辑。
type noopRecorder struct{}

// NewNoopRecorder 创建空 TraceRecorder。
func NewNoopRecorder() TraceRecorder { return noopRecorder{} }

func (noopRecorder) Record(_ *TraceRecord) {}

// mysqlRecorder 将 trace 同步写入 MySQL。
//
// 同步写入实现简单，避免引入后台 worker 和队列容量管理。写入失败只记录 Warn，
// 不影响用户的 chat 请求。
type mysqlRecorder struct {
	repo   TraceRepo
	logger *zap.Logger
}

// NewMySQLRecorder 创建同步落库的 TraceRecorder。
//
// logger 为 nil 时使用 zap.NewNop()，避免记录错误日志时 panic。
func NewMySQLRecorder(repo TraceRepo, logger *zap.Logger) TraceRecorder {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &mysqlRecorder{repo: repo, logger: logger}
}

// Record 同步写入 trace。写入失败仅记录 Warn 日志，不向主路径传播错误。
func (m *mysqlRecorder) Record(t *TraceRecord) {
	if err := m.repo.Insert(t); err != nil {
		m.logger.Warn("trace insert failed",
			zap.Int64("conversation_id", t.ConversationID),
			zap.Int64("user_id", t.UserID),
			zap.Error(err),
		)
	}
}
