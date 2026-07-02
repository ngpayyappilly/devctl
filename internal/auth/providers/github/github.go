package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"devctl/internal/auth"
)

const (
	name            = "github"
	defaultBaseURL  = "https://github.com"
	defaultAPIURL   = "https://api.github.com"
	grantType       = "urn:ietf:params:oauth:grant-type:device_code"
	defaultInterval = 5 * time.Second
)

var defaultScopes = []string{"read:user", "user:email"}

// Config holds GitHub OAuth App settings.
type Config struct {
	ClientID     string
	Scopes       []string // default: ["read:user", "user:email"]
	BaseURL      string   // default: "https://github.com"; override for GHE
	HTTPClient   *http.Client
	PollInterval time.Duration // overrides server interval; use time.Millisecond in tests
}

// Provider implements auth.Provider for GitHub OAuth Device Flow.
type Provider struct {
	cfg    Config
	apiURL string // "https://api.github.com" or "<BaseURL>/api/v3"
}

// New returns a GitHub provider with defaults applied.
func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = make([]string, len(defaultScopes))
		copy(cfg.Scopes, defaultScopes)
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	apiURL := defaultAPIURL
	if cfg.BaseURL != defaultBaseURL {
		// GitHub Enterprise Server uses <hostname>/api/v3.
		apiURL = strings.TrimRight(cfg.BaseURL, "/") + "/api/v3"
	}
	return &Provider{cfg: cfg, apiURL: apiURL}
}

func (p *Provider) Name() string { return name }

func (p *Provider) Login(ctx context.Context) (*auth.Session, error) {
	dc, err := p.requestDeviceCode(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "\nOpen the following URL in your browser:\n\n  %s\n\nEnter code: %s\n\nWaiting for authentication...\n",
		dc.VerificationURI, dc.UserCode)

	accessToken, err := p.pollForToken(ctx, dc)
	if err != nil {
		return nil, err
	}

	username, email, err := p.resolveUser(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("resolve GitHub user: %w", err)
	}

	s := &auth.Session{
		Provider:    name,
		AccessToken: accessToken,
		Username:    username,
		// ExpiresAt zero: GitHub tokens do not expire by default.
	}
	if email != "" {
		s.Extra = map[string]string{"email": email}
	}
	return s, nil
}

// Refresh is not supported — GitHub tokens do not expire.
func (p *Provider) Refresh(_ context.Context, _ *auth.Session) (*auth.Session, error) {
	return nil, auth.ErrRefreshNotSupported
}

func (p *Provider) Revoke(_ context.Context, _ *auth.Session) error {
	return nil
}

// --- internal types ---

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenPollResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
	Interval    int    `json:"interval"` // updated interval from slow_down
}

type userInfo struct {
	Login string `json:"login"`
	Email string `json:"email"`
}

// --- HTTP helpers ---

func (p *Provider) requestDeviceCode(ctx context.Context) (*deviceCodeResponse, error) {
	var dc deviceCodeResponse
	err := p.postJSON(ctx, p.cfg.BaseURL+"/login/device/code", map[string]any{
		"client_id": p.cfg.ClientID,
		"scope":     strings.Join(p.cfg.Scopes, " "),
	}, &dc)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	return &dc, nil
}

func (p *Provider) pollForToken(ctx context.Context, dc *deviceCodeResponse) (string, error) {
	interval := p.startInterval(dc.Interval)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		var r tokenPollResponse
		if err := p.postJSON(ctx, p.cfg.BaseURL+"/login/oauth/access_token", map[string]any{
			"client_id":   p.cfg.ClientID,
			"device_code": dc.DeviceCode,
			"grant_type":  grantType,
		}, &r); err != nil {
			return "", fmt.Errorf("poll token: %w", err)
		}

		switch r.Error {
		case "":
			return r.AccessToken, nil
		case "authorization_pending":
			// continue
		case "slow_down":
			if p.cfg.PollInterval == 0 {
				if r.Interval > 0 {
					interval = time.Duration(r.Interval) * time.Second
				} else {
					interval += 5 * time.Second
				}
			}
		case "expired_token":
			return "", fmt.Errorf("device code expired; run 'devctl auth login' again")
		case "access_denied":
			return "", fmt.Errorf("access denied by user")
		default:
			return "", fmt.Errorf("GitHub OAuth error: %s — %s", r.Error, r.ErrorDesc)
		}
	}
}

func (p *Provider) resolveUser(ctx context.Context, token string) (login, email string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/user", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var u userInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", "", err
	}
	return u.Login, u.Email, nil
}

func (p *Provider) postJSON(ctx context.Context, url string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(result)
}

func (p *Provider) startInterval(serverInterval int) time.Duration {
	if p.cfg.PollInterval > 0 {
		return p.cfg.PollInterval
	}
	if serverInterval > 0 {
		return time.Duration(serverInterval) * time.Second
	}
	return defaultInterval
}
