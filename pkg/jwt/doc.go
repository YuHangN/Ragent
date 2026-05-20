// Package jwt 封装业务 JWT 的签发和解析逻辑。
//
// 该包统一使用 HS256 签名算法，并在 token payload 中携带登录用户快照。
package jwt
