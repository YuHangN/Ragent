package user

import (
	"strings"

	"github.com/YuHangN/ragent-go/pkg/apperror"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

func (r Role) String() string { return string(r) }

// NormalizeRole 校验并规范化角色字符串。
func NormalizeRole(raw string) (Role, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RoleAdmin):
		return RoleAdmin, nil
	case string(RoleUser), "":
		return RoleUser, nil
	default:
		return "", apperror.NewClientMsg("角色类型不合法")
	}
}
