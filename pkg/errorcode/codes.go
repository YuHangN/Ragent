package errorcode

// 对齐 Java BaseErrorCode，遵循阿里巴巴错误码规范：
//   A 类：用户端错误（Client Error）
//   B 类：系统执行错误（Service Error）
//   C 类：第三方服务错误（Remote Error）

var (
	// ========== A 类：用户端错误 ==========

	ClientError = New("A000001", "用户端错误")

	// A01 用户注册错误
	UserRegisterError        = New("A000100", "用户注册错误")
	UserNameVerifyError      = New("A000110", "用户名校验失败")
	UserNameExistError       = New("A000111", "用户名已存在")
	UserNameSensitiveError   = New("A000112", "用户名包含敏感词")
	UserNameSpecialCharError = New("A000113", "用户名包含特殊字符")
	PasswordVerifyError      = New("A000120", "密码校验失败")
	PasswordShortError       = New("A000121", "密码长度不够")
	PhoneVerifyError         = New("A000151", "手机格式校验失败")

	// A02 幂等性错误
	IdempotentTokenNullError   = New("A000200", "幂等Token为空")
	IdempotentTokenDeleteError = New("A000201", "幂等Token已被使用或失效")

	// SearchAmountExceedsLimit A03 查询参数错误
	SearchAmountExceedsLimit = New("A000300", "查询数据量超过最大限制")

	Unauthorized = New("A000401", "未登录或登录已过期")
	Forbidden    = New("A000403", "权限不足")

	// ========== B 类：系统执行错误 ==========

	ServiceError        = New("B000001", "系统执行出错")
	ServiceTimeoutError = New("B000100", "系统执行超时")

	// ========== C 类：第三方服务错误 ==========

	RemoteError = New("C000001", "调用第三方服务出错")
)
