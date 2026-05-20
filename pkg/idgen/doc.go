// Package idgen 提供全局 Snowflake ID 生成器。
//
// 服务启动时应先调用 Init 初始化节点；业务代码随后可通过 NewID 或
// NewStringID 生成数据库主键和业务字符串 ID。
package idgen
