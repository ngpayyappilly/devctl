package ping

import (
	"context"
	"net/http"
	"time"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/oidc"
)

const name = "ping"

// Config holds PingID / PingFederate settings.
type Config struct {
	// IssuerURL is the full OIDC issuer URL, e.g. "https://ping.myorg.com/as".
	IssuerURL    string
	ClientID     string
	ClientSecret string // empty for public clients
	// HTTPClient and PollInterval are for testing only.
	HTTPClient   *http.Client
	PollInterval time.Duration
}

// Provider wraps OIDCProvider with PingID defaults:
//   - Uses the caller-supplied IssuerURL directly (no domain construction)
//   - Standard OIDC scopes (no Ping-specific extras by default)
//   - Name() returns "ping" so sessions are stored under the "ping" key
type Provider struct {
	inner *oidc.OIDCProvider
}

// New returns a PingID / PingFederate provider using the OIDC Device Flow.
func New(cfg Config) *Provider {
	return &Provider{
		inner: oidc.New(oidc.Config{
			IssuerURL:    cfg.IssuerURL,
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Scopes:       []string{"openid", "profile", "email", "offline_access"},
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
