package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/amoylab/unla/internal/common/config"
	"go.uber.org/zap"
)

// ExternalOAuth defines the interface for external OAuth providers
type ExternalOAuth interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*ExternalTokenResponse, error)
	GetUserInfo(ctx context.Context, accessToken string) (*ExternalUserInfo, error)
}

// ExternalTokenResponse represents the response from external OAuth token exchange
type ExternalTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ExternalUserInfo represents user information from external OAuth providers
type ExternalUserInfo struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	Picture  string `json:"picture,omitempty"`
	Provider string `json:"provider"`
}

// GoogleOAuth implements Google OAuth2 provider
type GoogleOAuth struct {
	logger       *zap.Logger
	clientID     string
	clientSecret string
	redirectURI  string
}

// NewGoogleOAuth creates a new Google OAuth provider
func NewGoogleOAuth(logger *zap.Logger, cfg config.GoogleOAuthConfig) *GoogleOAuth {
	return &GoogleOAuth{
		logger:       logger.Named("auth.google"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURI:  cfg.RedirectURI,
	}
}

// GetAuthURL returns the Google OAuth authorization URL
func (g *GoogleOAuth) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", g.clientID)
	params.Set("redirect_uri", g.redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// ExchangeCode exchanges authorization code for access token
func (g *GoogleOAuth) ExchangeCode(ctx context.Context, code string) (*ExternalTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", g.clientID)
	data.Set("client_secret", g.clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", g.redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", 
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		g.logger.Error("Google token exchange failed", 
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp ExternalTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// GetUserInfo retrieves user information from Google
func (g *GoogleOAuth) GetUserInfo(ctx context.Context, accessToken string) (*ExternalUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", 
		"https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		g.logger.Error("Google user info request failed",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return nil, fmt.Errorf("user info request failed with status %d", resp.StatusCode)
	}

	var googleUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &ExternalUserInfo{
		ID:       googleUser.ID,
		Email:    googleUser.Email,
		Name:     googleUser.Name,
		Picture:  googleUser.Picture,
		Provider: "google",
	}, nil
}

// GitHubOAuth implements GitHub OAuth2 provider
type GitHubOAuth struct {
	logger       *zap.Logger
	clientID     string
	clientSecret string
	redirectURI  string
}

// NewGitHubOAuth creates a new GitHub OAuth provider
func NewGitHubOAuth(logger *zap.Logger, cfg config.GitHubOAuthConfig) *GitHubOAuth {
	return &GitHubOAuth{
		logger:       logger.Named("auth.github"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURI:  cfg.RedirectURI,
	}
}

// GetAuthURL returns the GitHub OAuth authorization URL
func (gh *GitHubOAuth) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", gh.clientID)
	params.Set("redirect_uri", gh.redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "user:email")
	params.Set("state", state)
	
	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

// ExchangeCode exchanges authorization code for access token
func (gh *GitHubOAuth) ExchangeCode(ctx context.Context, code string) (*ExternalTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", gh.clientID)
	data.Set("client_secret", gh.clientSecret)
	data.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", 
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		gh.logger.Error("GitHub token exchange failed",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp ExternalTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// GetUserInfo retrieves user information from GitHub
func (gh *GitHubOAuth) GetUserInfo(ctx context.Context, accessToken string) (*ExternalUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		gh.logger.Error("GitHub user info request failed",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return nil, fmt.Errorf("user info request failed with status %d", resp.StatusCode)
	}

	var githubUser struct {
		ID       int    `json:"id"`
		Login    string `json:"login"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Avatar   string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// GitHub may not return email if it's private, need to fetch it separately
	email := githubUser.Email
	if email == "" {
		email, _ = gh.getUserEmail(ctx, accessToken)
	}

	return &ExternalUserInfo{
		ID:       fmt.Sprintf("%d", githubUser.ID),
		Email:    email,
		Name:     githubUser.Name,
		Username: githubUser.Login,
		Picture:  githubUser.Avatar,
		Provider: "github",
	}, nil
}

// getUserEmail fetches user's primary email from GitHub
func (gh *GitHubOAuth) getUserEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get user emails: %d", resp.StatusCode)
	}

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary {
			return email.Email, nil
		}
	}

	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", fmt.Errorf("no email found")
}