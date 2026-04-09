package user

import (
	"time"

	"gorm.io/gorm"
)

// User 对应数据库 t_user 表。
type User struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string         `gorm:"column:username;not null;uniqueIndex" json:"username"`
	Password  string         `gorm:"column:password;not null" json:"-"` // json:"-" 避免密码泄漏
	Avatar    string         `gorm:"column:avatar" json:"avatar"`
	Role      string         `gorm:"column:role;default:user" json:"role"`
	CreatedAt time.Time      `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdatedAt time.Time      `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted;index" json:"-"`
}

func (User) TableName() string { return "t_user" }

// LoginVO 是登录成功后返回给前端的数据。
type LoginVO struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	Token  string `json:"token"`
	Avatar string `json:"avatar"`
}

// CurrentUserVO 是 GET /user/me 的响应结构。
type CurrentUserVO struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Avatar   string `json:"avatar"`
}

// UserVO 是用户列表的响应结构（管理员用）。
type UserVO struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"createTime"`
	UpdatedAt time.Time `json:"updateTime"`
}

// PageResult 是分页响应的通用结构。
type PageResult[T any] struct {
	Total   int64 `json:"total"`
	Records []T   `json:"records"`
}
