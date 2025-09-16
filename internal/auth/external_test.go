package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newJSONResponse(status int, v any) *http.Response {
	b, _ := json.Marshal(v)
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(b))), Header: make(http.Header)}
}

func TestGoogleOAuth_Flow(t *testing.T) {
	logger := zap.NewNop()
	goauth := NewGoogleOAuth(logger, config.GoogleOAuthConfig{ClientID: "cid", ClientSecret: "sec", RedirectURI: "http://cb"})
	authURL := goauth.GetAuthURL("state123")
	assert.Contains(t, authURL, "client_id=cid")

	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()

	http.DefaultTransport = rtFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.String() == "https://oauth2.googleapis.com/token":
			_ = r.ParseForm()
			if r.PostForm.Get("code") == "good" {
				return newJSONResponse(200, map[string]any{"access_token": "at", "token_type": "Bearer"}), nil
			}
			return newJSONResponse(400, map[string]any{"error": "bad"}), nil
		case r.URL.String() == "https://www.googleapis.com/oauth2/v2/userinfo":
			if got := r.Header.Get("Authorization"); got == "Bearer at" {
				return newJSONResponse(200, map[string]any{"id": "1", "email": "e@x", "name": "n", "picture": "p"}), nil
			}
			return newJSONResponse(401, map[string]any{"error": "unauthorized"}), nil
		default:
			return newJSONResponse(404, map[string]any{"error": "not found"}), nil
		}
	})

	tok, err := goauth.ExchangeCode(context.Background(), "good")
	assert.NoError(t, err)
	assert.Equal(t, "at", tok.AccessToken)

	ui, err := goauth.GetUserInfo(context.Background(), tok.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, "google", ui.Provider)
	assert.Equal(t, "e@x", ui.Email)
}

func TestGitHubOAuth_Flow_WithEmailFallback(t *testing.T) {
	logger := zap.NewNop()
	ghauth := NewGitHubOAuth(logger, config.GitHubOAuthConfig{ClientID: "cid", ClientSecret: "sec", RedirectURI: "http://cb"})
	authURL := ghauth.GetAuthURL("state123")
	assert.Contains(t, authURL, "client_id=cid")

	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()

	http.DefaultTransport = rtFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.String() == "https://github.com/login/oauth/access_token":
			_ = r.ParseForm()
			if r.PostForm.Get("code") == "good" {
				return newJSONResponse(200, map[string]any{"access_token": "ghat", "token_type": "Bearer"}), nil
			}
			return newJSONResponse(400, map[string]any{"error": "bad"}), nil
		case r.URL.String() == "https://api.github.com/user":
			// simulate missing email -> fallback path
			return newJSONResponse(200, map[string]any{"id": 2, "login": "u", "name": "n", "avatar_url": "a"}), nil
		case r.URL.String() == "https://api.github.com/user/emails":
			return newJSONResponse(200, []map[string]any{{"email": "x@y", "primary": true}}), nil
		default:
			return newJSONResponse(404, map[string]any{"error": "not found"}), nil
		}
	})

	tok, err := ghauth.ExchangeCode(context.Background(), "good")
	assert.NoError(t, err)
	assert.Equal(t, "ghat", tok.AccessToken)

	ui, err := ghauth.GetUserInfo(context.Background(), tok.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, "github", ui.Provider)
	assert.Equal(t, "x@y", ui.Email)
	assert.Equal(t, "u", ui.Username)
}
