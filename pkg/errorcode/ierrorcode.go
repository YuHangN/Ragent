package errorcode

// 通过接口抽象，apperror 包可以接受任意业务自定义的错误码，而不依赖具体枚举。
type IErrorCode interface {
	Code() string
	Message() string
}

// Code 是 IErrorCode 的标准实现。
type Code struct {
	CodeStr    string
	MessageStr string
}

func (c Code) Code() string    { return c.CodeStr }
func (c Code) Message() string { return c.MessageStr }

// New 辅助构造函数。
func New(code, message string) Code {
	return Code{CodeStr: code, MessageStr: message}
}
