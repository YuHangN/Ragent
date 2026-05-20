package response

// Result 是所有 HTTP 接口的统一响应结构。
//
// 泛型参数 T 表示 data 字段的实际类型。
// 当接口无返回数据时，调用 Success[any](nil)。
type Result[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}

// Success 构造成功响应，code 固定为 "0"。
func Success[T any](data T) Result[T] {
	return Result[T]{
		Code: "0",
		Data: data,
	}
}

// Fail 构造失败响应。
func Fail[T any](code, message string) Result[T] {
	return Result[T]{
		Code:    code,
		Message: message,
	}
}
