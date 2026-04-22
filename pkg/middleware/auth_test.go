package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YuHangN/ragent-go/pkg/jwt"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const testSecret = "unit-test-secret"

func makeToken(t *testing.T, ttl time.Duration) string {
	t.Helper()
	tok, err := jwt.Sign(jwt.UserClaims{UserID: "42", Username: "alice", Role: "user"}, testSecret, ttl)
	assert.NoError(t, err)
	return tok
}

func authRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(testSecret))
	r.GET("/protected", func(c *gin.Context) {
		u := usercontext.Require(c)
		c.JSON(http.StatusOK, gin.H{"userID": u.UserID, "username": u.Username})
	})
	return r
}

func TestAuth_ValidToken(t *testing.T) {
	token := makeToken(t, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	authRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "42")
}

func TestAuth_MissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	authRouter().ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ExpiredToken(t *testing.T) {
	token := makeToken(t, -time.Second)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	authRouter().ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
