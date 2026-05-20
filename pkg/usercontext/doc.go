// Package usercontext 在 Gin 请求上下文中保存当前登录用户。
//
// Auth 中间件解析 JWT 后写入 LoginUser，业务 handler 可通过本包读取用户 ID、
// 用户名和角色。
package usercontext
