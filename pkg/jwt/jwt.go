package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserClaims 是写入 JWT payload 的登录用户信息。
type UserClaims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Avatar   string `json:"avatar"`
	jwt.RegisteredClaims
}

// Sign 使用 HS256 算法签发 JWT，并通过 ttl 控制有效期。
func Sign(claims UserClaims, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Parse 验证 JWT 签名和有效期，并返回其中的用户信息。
func Parse(tokenStr, secret string) (*UserClaims, error) {
	var claims UserClaims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return &claims, nil
}
