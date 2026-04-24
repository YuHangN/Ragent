package response

// PageResult 分页响应的通用结构，供所有业务模块复用。
// 对齐 Java IPage<T> 的最终序列化形态。
type PageResult[T any] struct {
	Total   int64 `json:"total"`
	Records []T   `json:"records"`
}
