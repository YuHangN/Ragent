package errorcode

// IErrorCode 抽象统一错误码，允许业务模块定义自己的错误码实现。
type IErrorCode interface {
	Code() string
	Message() string
}

// Code 是 IErrorCode 的基础实现。
type Code struct {
	CodeStr    string
	MessageStr string
}

// Code 返回错误码字符串。
func (c Code) Code() string { return c.CodeStr }

// Message 返回错误码对应的默认提示。
func (c Code) Message() string { return c.MessageStr }

// New 构造基础错误码。
func New(code, message string) Code {
	return Code{CodeStr: code, MessageStr: message}
}
