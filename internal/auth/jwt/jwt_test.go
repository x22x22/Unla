package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJWTService_GenerateAndValidate(t *testing.T) {
	s := NewService(Config{SecretKey: "secret", Duration: time.Hour})
	tok, err := s.GenerateToken(42, "alice", "admin")
	assert.NoError(t, err)
	claims, err := s.ValidateToken(tok)
	assert.NoError(t, err)
	if assert.NotNil(t, claims) {
		assert.Equal(t, uint(42), claims.UserID)
		assert.Equal(t, "alice", claims.Username)
		assert.Equal(t, "admin", claims.Role)
	}
}

func TestJWTService_ExpiredAndInvalid(t *testing.T) {
	s := NewService(Config{SecretKey: "secret", Duration: -time.Second})
	tok, err := s.GenerateToken(1, "bob", "user")
	assert.NoError(t, err)
	// Token should be expired immediately
	claims, err := s.ValidateToken(tok)
	assert.Nil(t, claims)
	assert.Error(t, err)

	// Invalid token string
	claims, err = s.ValidateToken("not-a-token")
	assert.Nil(t, claims)
	assert.Error(t, err)
}
