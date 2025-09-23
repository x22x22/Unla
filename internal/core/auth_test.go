package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/internal/auth"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/common/errorx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockAuthService struct {
	serverMetadata func(r *http.Request) map[string]interface{}
	authorize      func(ctx context.Context, r *http.Request) (*auth.AuthorizationResponse, error)
	token          func(ctx context.Context, r *http.Request) (*auth.TokenResponse, error)
	register       func(ctx context.Context, r *http.Request) (*auth.ClientRegistrationResponse, error)
	revoke         func(ctx context.Context, r *http.Request) error
	validateToken  func(ctx context.Context, token string) error
}

func (m *mockAuthService) ServerMetadata(r *http.Request) map[string]interface{} {
	if m.serverMetadata != nil {
		return m.serverMetadata(r)
	}
	return map[string]interface{}{"issuer": "test"}
}

func (m *mockAuthService) Authorize(ctx context.Context, r *http.Request) (*auth.AuthorizationResponse, error) {
	if m.authorize != nil {
		return m.authorize(ctx, r)
	}
	return &auth.AuthorizationResponse{Code: "test_code", State: "test_state"}, nil
}

func (m *mockAuthService) Token(ctx context.Context, r *http.Request) (*auth.TokenResponse, error) {
	if m.token != nil {
		return m.token(ctx, r)
	}
	return &auth.TokenResponse{AccessToken: "test_token", TokenType: "Bearer"}, nil
}

func (m *mockAuthService) Register(ctx context.Context, r *http.Request) (*auth.ClientRegistrationResponse, error) {
	if m.register != nil {
		return m.register(ctx, r)
	}
	return &auth.ClientRegistrationResponse{ClientID: "test_client", ClientSecret: "test_secret"}, nil
}

func (m *mockAuthService) Revoke(ctx context.Context, r *http.Request) error {
	if m.revoke != nil {
		return m.revoke(ctx, r)
	}
	return nil
}

func (m *mockAuthService) ValidateToken(ctx context.Context, token string) error {
	if m.validateToken != nil {
		return m.validateToken(ctx, token)
	}
	if token == "valid_token" {
		return nil
	}
	return errorx.ErrTokenNotFound
}

// Additional methods to implement the full Auth interface
func (m *mockAuthService) IsOAuth2Enabled() bool {
	return true
}

func (m *mockAuthService) GetOAuth2CORS() *config.CORSConfig {
	return nil
}

func (m *mockAuthService) GetGoogleOAuth() auth.ExternalOAuth {
	return nil
}

func (m *mockAuthService) GetGitHubOAuth() auth.ExternalOAuth {
	return nil
}

func (m *mockAuthService) IsGoogleOAuthEnabled() bool {
	return false
}

func (m *mockAuthService) IsGitHubOAuthEnabled() bool {
	return false
}

func TestServer_handleOAuthServerMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := &mockAuthService{
		serverMetadata: func(r *http.Request) map[string]interface{} {
			return map[string]interface{}{
				"issuer":                 "https://example.com",
				"authorization_endpoint": "https://example.com/auth",
				"token_endpoint":         "https://example.com/token",
			}
		},
	}

	server := &Server{
		auth:   mockAuth,
		logger: zap.NewNop(),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/oauth/metadata", nil)

	server.handleOAuthServerMetadata(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "https://example.com")
	assert.Contains(t, w.Body.String(), "authorization_endpoint")
}

func TestServer_handleOAuthToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful token request", func(t *testing.T) {
		mockAuth := &mockAuthService{
			token: func(ctx context.Context, r *http.Request) (*auth.TokenResponse, error) {
				return &auth.TokenResponse{
					AccessToken: "access_token",
					TokenType:   "Bearer",
					ExpiresIn:   3600,
				}, nil
			},
		}

		server := &Server{
			auth:   mockAuth,
			logger: zap.NewNop(),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/oauth/token", nil)

		server.handleOAuthToken(c)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "access_token")
		assert.Contains(t, w.Body.String(), "Bearer")
	})

	t.Run("token request error", func(t *testing.T) {
		mockAuth := &mockAuthService{
			token: func(ctx context.Context, r *http.Request) (*auth.TokenResponse, error) {
				return nil, errorx.ErrInvalidClient
			},
		}

		server := &Server{
			auth:   mockAuth,
			logger: zap.NewNop(),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/oauth/token", nil)

		server.handleOAuthToken(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "error")
	})
}

func TestServer_handleOAuthRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful registration", func(t *testing.T) {
		mockAuth := &mockAuthService{
			register: func(ctx context.Context, r *http.Request) (*auth.ClientRegistrationResponse, error) {
				return &auth.ClientRegistrationResponse{
					ClientID:     "client123",
					ClientSecret: "secret456",
				}, nil
			},
		}

		server := &Server{
			auth:   mockAuth,
			logger: zap.NewNop(),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/oauth/register", nil)

		server.handleOAuthRegister(c)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "client123")
	})
}

func TestServer_handleOAuthRevoke(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful revocation", func(t *testing.T) {
		mockAuth := &mockAuthService{
			revoke: func(ctx context.Context, r *http.Request) error {
				return nil
			},
		}

		server := &Server{
			auth:   mockAuth,
			logger: zap.NewNop(),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/oauth/revoke", nil)

		server.handleOAuthRevoke(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestServer_isValidAccessToken(t *testing.T) {
	mockAuth := &mockAuthService{
		validateToken: func(ctx context.Context, token string) error {
			if token == "valid_token" {
				return nil
			}
			return errorx.ErrTokenNotFound
		},
	}

	server := &Server{
		auth:   mockAuth,
		logger: zap.NewNop(),
	}

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid_token")

		result := server.isValidAccessToken(req)
		assert.True(t, result)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid_token")

		result := server.isValidAccessToken(req)
		assert.False(t, result)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		result := server.isValidAccessToken(req)
		assert.False(t, result)
	})

	t.Run("invalid authorization format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")

		result := server.isValidAccessToken(req)
		assert.False(t, result)
	})

	t.Run("missing bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer ")

		result := server.isValidAccessToken(req)
		assert.False(t, result)
	})
}

func TestServer_sendOAuthError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := &Server{
		logger: zap.NewNop(),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	server.sendOAuthError(c, errorx.ErrInvalidClient)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "error")
	assert.Contains(t, w.Body.String(), "invalid_client")
}
