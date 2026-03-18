package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthConfig holds per-provider OAuth2 settings.
type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	RedirectBaseURL    string
}

// OAuthUserInfo is the minimal profile returned after OAuth login.
type OAuthUserInfo struct {
	Provider       string
	ProviderUserID string
	Email          string
	DisplayName    string
	AvatarURL      string
}

// OAuthService wraps per-provider oauth2.Config instances.
type OAuthService struct {
	google *oauth2.Config
	github *oauth2.Config
}

// NewOAuthService constructs the OAuth service with provider configs.
func NewOAuthService(cfg OAuthConfig) *OAuthService {
	return &OAuthService{
		google: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.RedirectBaseURL + "/auth/oauth/google/callback",
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint:     google.Endpoint,
		},
		github: &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.RedirectBaseURL + "/auth/oauth/github/callback",
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
	}
}

// AuthURL returns the provider's OAuth2 authorization URL with a CSRF state token.
func (s *OAuthService) AuthURL(provider, state string) (string, error) {
	cfg, err := s.configFor(provider)
	if err != nil {
		return "", err
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

// Exchange swaps the authorization code for a token and fetches the user's profile.
func (s *OAuthService) Exchange(ctx context.Context, provider, code string) (*OAuthUserInfo, error) {
	cfg, err := s.configFor(provider)
	if err != nil {
		return nil, err
	}
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}

	client := cfg.Client(ctx, token)
	switch provider {
	case "google":
		return fetchGoogleUser(client)
	case "github":
		return fetchGitHubUser(client)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (s *OAuthService) configFor(provider string) (*oauth2.Config, error) {
	switch provider {
	case "google":
		return s.google, nil
	case "github":
		return s.github, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func fetchGoogleUser(client *http.Client) (*OAuthUserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode google userinfo: %w", err)
	}
	return &OAuthUserInfo{
		Provider:       "google",
		ProviderUserID: data.Sub,
		Email:          data.Email,
		DisplayName:    data.Name,
		AvatarURL:      data.Picture,
	}, nil
}

func fetchGitHubUser(client *http.Client) (*OAuthUserInfo, error) {
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("github user: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode github user: %w", err)
	}

	displayName := data.Name
	if displayName == "" {
		displayName = data.Login
	}

	// GitHub may return empty email; fetch the primary email separately.
	email := data.Email
	if email == "" {
		email, _ = fetchGitHubPrimaryEmail(client)
	}

	return &OAuthUserInfo{
		Provider:       "github",
		ProviderUserID: fmt.Sprintf("%d", data.ID),
		Email:          email,
		DisplayName:    displayName,
		AvatarURL:      data.AvatarURL,
	}, nil
}

func fetchGitHubPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no primary email found")
}
