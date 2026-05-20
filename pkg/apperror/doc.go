// Package apperror 定义应用层统一错误类型。
//
// 该包把错误分为客户端错误、服务端错误和远端依赖错误三类，并携带统一错误码、
// 可展示消息和底层 cause，方便 middleware 映射 HTTP 状态码，也方便业务代码
// 使用 errors.Is / errors.As 追溯原始错误。
package apperror
