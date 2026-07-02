package apikey

import (
	"context"
	"fmt"
	"os"
	"time"

	"devctl/internal/auth"
	"devctl/pkg/config"
)

const name = "apikey"

// Provider authenticates non-interactively using a static token. Suitable for
// CI/CD pipelines and service accounts. The token is resolved in priority order:
//  1. flagToken (set from --token CLI flag before calling Login)
//  2. DEVCTL_TOKEN environment variable
//  3. auth.token config key in .devctl.yaml / ~/.devctl/config.yaml
type Provider struct {
	flagToken string
}

// New returns a Provider. Pass the value of the --token flag if present;
// use "" when relying on env var or config.
func New(flagToken string) *Provider {
	return &Provider{flagToken: flagToken}
}

func (p *Provider) Name() string { return name }

func (p *Provider) Login(_ context.Context) (*auth.Session, error) {
	tok := p.resolve()
	if tok == "" {
		return nil, fmt.Errorf(
			"no API key found: set DEVCTL_TOKEN env var, auth.token in config, or pass --token",
		)
	}
	return &auth.Session{
		Provider:    name,
		AccessToken: tok,
		Username:    "service-account",
		ExpiresAt:   time.Time{}, // zero = never expires
	}, nil
}

// Refresh is not supported — static tokens don't expire.
func (p *Provider) Refresh(_ context.Context, _ *auth.Session) (*auth.Session, error) {
	return nil, auth.ErrRefreshNotSupported
}

// Revoke is a no-op — static tokens have no server-side revocation endpoint.
func (p *Provider) Revoke(_ context.Context, _ *auth.Session) error {
	return nil
}

// resolve returns the first non-empty token value across all sources.
func (p *Provider) resolve() string {
	if p.flagToken != "" {
		return p.flagToken
	}
	if v := os.Getenv("DEVCTL_TOKEN"); v != "" {
		return v
	}
	return config.GetString(config.KeyAuthToken, "")
}
