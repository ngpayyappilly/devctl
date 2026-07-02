package okta

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/oidc"
)

const name = "okta"

// Config holds Okta-specific settings.
type Config struct {
	// Domain is the Okta org domain, e.g. "myorg.okta.com".
	Domain       string
	ClientID     string
	ClientSecret string // empty for public clients
	// IssuerURL overrides the computed "https://<Domain>" issuer URL.
	// Intended for tests pointing at an HTTP mock server.
	IssuerURL    string
	// HTTPClient and PollInterval are for testing only.
	HTTPClient   *http.Client
	PollInterval time.Duration
}

// Provider wraps OIDCProvider with Okta-specific defaults:
//   - IssuerURL = https://<Domain>
//   - Scopes include "groups" for group membership claims
//   - Name() returns "okta" so sessions are stored under the "okta" key
type Provider struct {
	inner *oidc.OIDCProvider
}

// New returns an Okta provider. The OIDC Device Flow is used for login.
func New(cfg Config) *Provider {
	issuerURL := cfg.IssuerURL
	if issuerURL == "" {
		issuerURL = fmt.Sprintf("https://%s", cfg.Domain)
	}
	return &Provider{
		inner: oidc.New(oidc.Config{
			IssuerURL:    issuerURL,
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Scopes:       []string{"openid", "profile", "email", "offline_access", "groups"},
			HTTPClient:   cfg.HTTPClient,
			PollInterval: cfg.PollInterval,
		}),
	}
}

func (p *Provider) Name() string { return name }

func (p *Provider) Login(ctx context.Context) (*auth.Session, error) {
	s, err := p.inner.Login(ctx)
	if err != nil {
		return nil, err
	}
	s.Provider = name
	return s, nil
}

func (p *Provider) Refresh(ctx context.Context, s *auth.Session) (*auth.Session, error) {
	return p.inner.Refresh(ctx, s)
}

func (p *Provider) Revoke(ctx context.Context, s *auth.Session) error {
	return p.inner.Revoke(ctx, s)
}
