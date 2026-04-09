package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignAndParse_Success(t *testing.T) {
	secret := "test-secret"
	claims := UserClaims{UserID: "123", Username: "alice", Role: "admin"}

	token, err := Sign(claims, secret, time.Hour)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := Parse(token, secret)
	assert.NoError(t, err)
	assert.Equal(t, "123", parsed.UserID)
	assert.Equal(t, "alice", parsed.Username)
	assert.Equal(t, "admin", parsed.Role)
}

func TestParse_WrongSecret(t *testing.T) {
	claims := UserClaims{UserID: "1", Username: "bob", Role: "user"}
	token, _ := Sign(claims, "secret-a", time.Hour)

	_, err := Parse(token, "secret-b")
	assert.Error(t, err)
}

func TestParse_Expired(t *testing.T) {
	claims := UserClaims{UserID: "1", Username: "bob", Role: "user"}
	token, _ := Sign(claims, "secret", -time.Second) // 已过期

	_, err := Parse(token, "secret")
	assert.Error(t, err)
}
