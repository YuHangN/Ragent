package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, Role("admin"), RoleAdmin)
	assert.Equal(t, Role("user"), RoleUser)
}

func TestNormalizeRole(t *testing.T) {
	cases := []struct {
		input    string
		expected Role
		hasErr   bool
	}{
		{"admin", RoleAdmin, false},
		{"ADMIN", RoleAdmin, false},
		{"  admin  ", RoleAdmin, false},
		{"user", RoleUser, false},
		{"", RoleUser, false},
		{"root", "", true},
		{"superadmin", "", true},
	}

	for _, c := range cases {
		got, err := NormalizeRole(c.input)
		if c.hasErr {
			assert.Error(t, err, "input=%q", c.input)
		} else {
			assert.NoError(t, err, "input=%q", c.input)
			assert.Equal(t, c.expected, got, "input=%q", c.input)
		}
	}
}
