// Package admin 实现 RAG 系统的运维侧能力：链路追踪、概览统计、运维工具。
//
// 本文件提供 TraceRecorder——ChatService 调用的链路追踪入口。它的核心价值是
// 解耦：chat 主路径只依赖一个 Record 方法，不关心 trace 落 MySQL 还是被丢弃。
package admin

import "go.uber.org/zap"

// TraceRecorder 是 ChatService 调用的链路追踪入口。
//
// 设计要点：
//   - Record 不返回 error——trace 失败绝不能阻断 chat 主路径，错误内部消化
//   - 两个实现：noopRecorder（trace 关闭时）/ mysqlRecorder（trace 开启时），
//     由 main.go 按 cfg.RAG.Trace.Enabled 选择注入
type TraceRecorder interface {
	Record(t *TraceRecord)
}

// noopRecorder 不做任何事，供 cfg.RAG.Trace.Enabled=false 时使用。
//
// 用空实现而不是在 ChatService 里写 `if recorder != nil`——调用方永远拿到一个
// 非 nil 的 recorder，主路径代码不必到处判空。
type noopRecorder struct{}

// NewNoopRecorder 构造空实现。
func NewNoopRecorder() TraceRecorder { return noopRecorder{} }

func (noopRecorder) Record(_ *TraceRecord) {}

// mysqlRecorder 同步写 MySQL。
//
// MVP 选择同步写：实现简单，无后台 worker，无 channel 容量考量。代价是每次
// chat 请求多一次 INSERT 的延迟（通常 <2ms，相对动辄数百 ms 的 LLM 调用可忽略）。
// trace 失败用 zap.Warn 报告，主路径继续。Phase X 扫尾时若发现写入压力大，
// 再升级成带 buffer 的异步 worker。
type mysqlRecorder struct {
	repo   TraceRepo
	logger *zap.Logger
}

// NewMySQLRecorder 构造同步落库实现。logger 为 nil 时退化为 zap.NewNop()，
// 不会 panic。
func NewMySQLRecorder(repo TraceRepo, logger *zap.Logger) TraceRecorder {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &mysqlRecorder{repo: repo, logger: logger}
}

// Record 同步落库。错误吞掉，仅打 Warn 日志——这是显式契约：trace 是旁路观测，
// 失败不应让用户的 chat 请求受影响。
func (m *mysqlRecorder) Record(t *TraceRecord) {
	if err := m.repo.Insert(t); err != nil {
		m.logger.Warn("trace insert failed",
			zap.Int64("conversation_id", t.ConversationID),
			zap.Int64("user_id", t.UserID),
			zap.Error(err),
		)
	}
}