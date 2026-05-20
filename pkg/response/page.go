package response

// PageResult 是分页响应的通用结构。
type PageResult[T any] struct {
	Total   int64 `json:"total"`
	Records []T   `json:"records"`
}
