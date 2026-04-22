package user

// LoginRequest 对应 Java LoginRequest。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest 对应 Java ChangePasswordRequest。
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}

// UserCreateRequest 对应 Java UserCreateRequest。
type UserCreateRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
	Avatar   string `json:"avatar"`
}

// UserUpdateRequest 对应 Java UserUpdateRequest。
// 使用指针区分"未传字段"和"传空串"，对齐 Java 的 null 语义。
type UserUpdateRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
	Avatar   *string `json:"avatar"`
}

// UserPageRequest 对应 Java UserPageRequest（继承 Page）。
// Gin 通过 `form` tag 从 query string 绑定。
type UserPageRequest struct {
	Current int    `form:"current,default=1"`
	Size    int    `form:"size,default=20"`
	Keyword string `form:"keyword"`
}
