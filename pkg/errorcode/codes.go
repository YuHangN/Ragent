package errorcode

// 错误码规范：A 开头客户端错误，B 开头服务端错误，对齐 Java 端 BaseErrorCode。
const (
	// ClientError 通用客户端错误
	ClientError = "A000001"
	// ServiceError 通用服务端错误
	ServiceError = "B000001"
	// Unauthorized 未登录或登录已过期
	Unauthorized = "A000200"
	// Forbidden 权限不足
	Forbidden = "A000300"
)
