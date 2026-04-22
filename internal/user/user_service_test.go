package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPasswordHash(t *testing.T) {
	hashed, err := HashPassword("mypassword")
	assert.NoError(t, err)
	assert.NotEqual(t, "mypassword", hashed)
	assert.True(t, CheckPassword("mypassword", hashed))
	assert.False(t, CheckPassword("wrongpassword", hashed))
}
