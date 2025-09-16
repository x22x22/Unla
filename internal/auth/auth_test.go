package auth

import (
	"context"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/common/errorx"
	"go.uber.org/zap"
)

func TestNewAuth_DisabledAndMethods(t *testing.T) {
	logger := zap.NewNop()
	a, err := NewAuth(logger, config.AuthConfig{})
	if err != nil {
		t.Fatalf("NewAuth error: %v", err)
	}

	if a.IsOAuth2Enabled() {
		t.Fatalf("expected oauth2 disabled")
	}
	if a.GetOAuth2CORS() != nil {
		t.Fatalf("expected nil CORS when not configured")
	}
	if err := a.ValidateToken(context.Background(), "atk"); err == nil || err != errorx.ErrOAuth2NotEnabled {
		t.Fatalf("expected ErrOAuth2NotEnabled, got %v", err)
	}
	if a.IsGoogleOAuthEnabled() {
		t.Fatalf("google oauth should be disabled by default")
	}
	if a.IsGitHubOAuthEnabled() {
		t.Fatalf("github oauth should be disabled by default")
	}
}

func TestNewAuth_WithProviders(t *testing.T) {
	logger := zap.NewNop()
	cfg := config.AuthConfig{
		OAuth2: &config.OAuth2Config{Issuer: "http://local", Storage: config.OAuth2StorageConfig{Type: "memory"}},
		CORS:   &config.CORSConfig{AllowOrigins: []string{"*"}},
		Google: &config.GoogleOAuthConfig{ClientID: "cid", ClientSecret: "sec", RedirectURI: "http://cb"},
		GitHub: &config.GitHubOAuthConfig{ClientID: "gid", ClientSecret: "gsec", RedirectURI: "http://cb"},
	}
	a, err := NewAuth(logger, cfg)
	if err != nil {
		t.Fatalf("NewAuth error: %v", err)
	}
	if !a.IsOAuth2Enabled() {
		t.Fatalf("expected oauth2 enabled")
	}
	if a.GetOAuth2CORS() == nil {
		t.Fatalf("expected non-nil CORS config")
	}
	if a.GetGoogleOAuth() == nil || !a.IsGoogleOAuthEnabled() {
		t.Fatalf("expected google oauth enabled")
	}
	if a.GetGitHubOAuth() == nil || !a.IsGitHubOAuthEnabled() {
		t.Fatalf("expected github oauth enabled")
	}

	// When client secret empty, IsGoogleOAuthEnabled should be false
	cfg2 := cfg
	cfg2.Google = &config.GoogleOAuthConfig{ClientID: "cid"}
	a2, err := NewAuth(logger, cfg2)
	if err != nil {
		t.Fatalf("NewAuth error: %v", err)
	}
	if a2.IsGoogleOAuthEnabled() {
		t.Fatalf("expected google oauth disabled with missing secret")
	}
}
