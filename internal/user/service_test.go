package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizeRole 测试角色规范化逻辑
func TestNormalizeRole(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		hasErr   bool
	}{
		{"admin", "admin", false},
		{"ADMIN", "admin", false},
		{"user", "user", false},
		{"", "user", false}, // 默认 user
		{"root", "", true},  // 非法角色
	}
	for _, c := range cases {
		role, err := NormalizeRole(c.input)
		if c.hasErr {
			assert.Error(t, err, "input=%s", c.input)
		} else {
			assert.NoError(t, err, "input=%s", c.input)
			assert.Equal(t, c.expected, role, "input=%s", c.input)
		}
	}
}

// TestPasswordHash 测试 bcrypt 哈希和验证
func TestPasswordHash(t *testing.T) {
	hashed, err := HashPassword("mypassword")
	assert.NoError(t, err)
	assert.NotEqual(t, "mypassword", hashed)

	assert.True(t, CheckPassword("mypassword", hashed))
	assert.False(t, CheckPassword("wrongpassword", hashed))
}
