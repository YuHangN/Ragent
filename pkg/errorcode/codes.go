package errorcode

// 基础错误码遵循阿里巴巴错误码规范：
//   A 类：用户端错误（Client Error）
//   B 类：系统执行错误（Service Error）
//   C 类：第三方服务错误（Remote Error）

var (
	// ========== A 类：用户端错误 ==========

	// ClientError 表示通用用户端错误。
	ClientError = New("A000001", "用户端错误")

	// A01 用户注册错误
	// UserRegisterError 表示用户注册流程失败。
	UserRegisterError = New("A000100", "用户注册错误")
	// UserNameVerifyError 表示用户名校验失败。
	UserNameVerifyError = New("A000110", "用户名校验失败")
	// UserNameExistError 表示用户名已被占用。
	UserNameExistError = New("A000111", "用户名已存在")
	// UserNameSensitiveError 表示用户名包含敏感词。
	UserNameSensitiveError = New("A000112", "用户名包含敏感词")
	// UserNameSpecialCharError 表示用户名包含非法特殊字符。
	UserNameSpecialCharError = New("A000113", "用户名包含特殊字符")
	// PasswordVerifyError 表示密码校验失败。
	PasswordVerifyError = New("A000120", "密码校验失败")
	// PasswordShortError 表示密码长度不足。
	PasswordShortError = New("A000121", "密码长度不够")
	// PhoneVerifyError 表示手机号格式校验失败。
	PhoneVerifyError = New("A000151", "手机格式校验失败")

	// A02 幂等性错误
	// IdempotentTokenNullError 表示幂等 token 缺失。
	IdempotentTokenNullError = New("A000200", "幂等Token为空")
	// IdempotentTokenDeleteError 表示幂等 token 已被使用或失效。
	IdempotentTokenDeleteError = New("A000201", "幂等Token已被使用或失效")

	// SearchAmountExceedsLimit 表示查询数据量超过允许上限。
	SearchAmountExceedsLimit = New("A000300", "查询数据量超过最大限制")

	// Unauthorized 表示未登录或登录态已失效。
	Unauthorized = New("A000401", "未登录或登录已过期")
	// Forbidden 表示当前用户没有访问权限。
	Forbidden = New("A000403", "权限不足")

	// ========== B 类：系统执行错误 ==========

	// ServiceError 表示通用系统执行错误。
	ServiceError = New("B000001", "系统执行出错")
	// ServiceTimeoutError 表示系统执行超时。
	ServiceTimeoutError = New("B000100", "系统执行超时")

	// ========== C 类：第三方服务错误 ==========

	// RemoteError 表示调用第三方服务出错。
	RemoteError = New("C000001", "调用第三方服务出错")
)
