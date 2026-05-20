// Package aiclient 提供与具体供应商解耦的 AI 客户端和路由服务。
//
// 该包把聊天、向量化和重排能力收敛到小接口后面，再通过配置中的模型候选、
// 健康状态和 fallback 规则选择实际调用目标。各 provider client 只负责协议适配，
// 路由、URL 解析和熔断逻辑则统一留在包内共享代码中。
package aiclient
