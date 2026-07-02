package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"devctl/internal/auth"
)

const name = "oidc"

// Config holds the OIDC provider configuration.
type Config struct {
	IssuerURL    string   // e.g. https://myorg.okta.com
	ClientID     string
	ClientSecret string   // empty for public clients (PKCE)
	Scopes       []string // defaults to [openid profile email offline_access]
	// HTTPClient allows injection for testing; nil uses http.DefaultClient.
	HTTPClient *http.Client
	// PollInterval overrides the IdP-suggested polling interval.
	// Zero means use the IdP's interval (minimum 5 s per RFC 8628).
	// Set to a small value (e.g. time.Millisecond) in tests.
	PollInterval time.Duration
}

// OIDCProvider authenticates via the OAuth 2.0 Device Authorization Flow
// (RFC 8628). Works with any OIDC-compliant IdP that advertises a
// device_authorization_endpoint in its discovery document.
type OIDCProvider struct {
	cfg Config
}

// New returns an OIDCProvider. Scopes default to openid, profile, email,
// offline_access if not specified.
func New(cfg Config) *OIDCProvider {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email", "offline_access"}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &OIDCProvider{cfg: cfg}
}

func (p *OIDCProvider) Name() string { return name }

// --- OIDC discovery ---

type discoveryDoc struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
	RevocationEndpoint          string `json:"revocation_endpoint"`
	UserinfoEndpoint            string `json:"userinfo_endpoint"`
}

func (p *OIDCProvider) discover(ctx context.Context) (*discoveryDoc, error) {
	wellKnown := strings.TrimSuffix(p.cfg.IssuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery: HTTP %d from %s", resp.StatusCode, wellKnown)
	}
	var doc discoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("OIDC discovery decode: %w", err)
	}
	if doc.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf(
			"IdP at %s does not support Device Authorization Flow (missing device_authorization_endpoint)",
			p.cfg.IssuerURL,
		)
	}
	return &doc, nil
}

// --- Device Authorization Flow ---

type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

func (p *OIDCProvider) requestDeviceCode(ctx context.Context, doc *discoveryDoc) (*deviceAuthResponse, error) {
	body := url.Values{
		"client_id": {p.cfg.ClientID},
		"scope":     {strings.Join(p.cfg.Scopes, " ")},
	}
	if p.cfg.ClientSecret != "" {
		body.Set("client_secret", p.cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		doc.DeviceAuthorizationEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device authorization request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device authorization: HTTP %d: %s", resp.StatusCode, b)
	}
	var dar deviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&dar); err != nil {
		return nil, fmt.Errorf("device authorization decode: %w", err)
	}
	if dar.Interval == 0 {
		dar.Interval = 5 // RFC 8628 default
	}
	return &dar, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (p *OIDCProvider) pollForToken(ctx context.Context, doc *discoveryDoc, dar *deviceAuthResponse) (*tokenResponse, error) {
	interval := time.Duration(dar.Interval) * time.Second
	if p.cfg.PollInterval > 0 {
		interval = p.cfg.PollInterval
	}
	deadline := time.Now().Add(time.Duration(dar.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return nil, errors.New("device code expired; run 'devctl auth login' to try again")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		body := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {dar.DeviceCode},
			"client_id":   {p.cfg.ClientID},
		}
		if p.cfg.ClientSecret != "" {
			body.Set("client_secret", p.cfg.ClientSecret)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			doc.TokenEndpoint, strings.NewReader(body.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := p.cfg.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("token poll: %w", err)
		}
		var tr tokenResponse
		json.NewDecoder(resp.Body).Decode(&tr) //nolint:errcheck
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return &tr, nil
		}

		switch tr.Error {
		case "authorization_pending":
			// keep polling at current interval
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return nil, errors.New("device code expired; run 'devctl auth login' to try again")
		case "access_denied":
			return nil, errors.New("authentication denied by user")
		default:
			return nil, fmt.Errorf("token error %q: %s", tr.Error, tr.ErrorDesc)
		}
	}
}

// --- Userinfo ---

type userinfoClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (p *OIDCProvider) resolveUsername(ctx context.Context, doc *discoveryDoc, accessToken string) string {
	if doc.UserinfoEndpoint == "" {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, doc.UserinfoEndpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()
	var claims userinfoClaims
	json.NewDecoder(resp.Body).Decode(&claims) //nolint:errcheck
	if claims.Email != "" {
		return claims.Email
	}
	if claims.Name != "" {
		return claims.Name
	}
	return claims.Sub
}

// --- Provider interface ---

func (p *OIDCProvider) Login(ctx context.Context) (*auth.Session, error) {
	doc, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}
	dar, err := p.requestDeviceCode(ctx, doc)
	if err != nil {
		return nil, err
	}

	verifyURI := dar.VerificationURIComplete
	if verifyURI == "" {
		verifyURI = dar.VerificationURI
	}
	fmt.Printf("\nOpen the following URL in your browser:\n\n  %s\n\nEnter code: %s\n\nWaiting for authentication...\n\n",
		verifyURI, dar.UserCode)

	tr, err := p.pollForToken(ctx, doc, dar)
	if err != nil {
		return nil, err
	}

	username := p.resolveUsername(ctx, doc, tr.AccessToken)
	if username == "" {
		username = p.cfg.ClientID
	}

	var expiresAt time.Time
	if tr.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	return &auth.Session{
		Provider:     p.Name(),
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		IDToken:      tr.IDToken,
		ExpiresAt:    expiresAt,
		Username:     username,
	}, nil
}

func (p *OIDCProvider) Refresh(ctx context.Context, s *auth.Session) (*auth.Session, error) {
	if s.RefreshToken == "" {
		return nil, auth.ErrRefreshNotSupported
	}
	doc, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}

	body := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {s.RefreshToken},
		"client_id":     {p.cfg.ClientID},
	}
	if p.cfg.ClientSecret != "" {
		body.Set("client_secret", p.cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		doc.TokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	defer resp.Body.Close()

	var tr tokenResponse
	json.NewDecoder(resp.Body).Decode(&tr) //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed: %s: %s", tr.Error, tr.ErrorDesc)
	}
	if tr.RefreshToken == "" {
		tr.RefreshToken = s.RefreshToken // keep old token if not rotated
	}

	var expiresAt time.Time
	if tr.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	return &auth.Session{
		Provider:     p.Name(),
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		IDToken:      tr.IDToken,
		ExpiresAt:    expiresAt,
		Username:     s.Username, // preserve from original session
	}, nil
}

// Revoke is best-effort — no error returned even if revocation fails.
func (p *OIDCProvider) Revoke(ctx context.Context, s *auth.Session) error {
	doc, err := p.discover(ctx)
	if err != nil || doc.RevocationEndpoint == "" {
		return nil
	}
	body := url.Values{
		"token":           {s.AccessToken},
		"token_type_hint": {"access_token"},
		"client_id":       {p.cfg.ClientID},
	}
	if p.cfg.ClientSecret != "" {
		body.Set("client_secret", p.cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		doc.RevocationEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.cfg.HTTPClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	return nil
}
