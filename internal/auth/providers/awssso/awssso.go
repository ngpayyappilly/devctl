package awssso

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"

	"devctl/internal/auth"
)

const (
	name            = "aws-sso"
	clientName      = "devctl"
	clientType      = "public"
	grantType       = "urn:ietf:params:oauth:grant-type:device_code"
	sessionTTL      = 8 * time.Hour
	defaultInterval = 5 * time.Second
)

// oidcAPI is the subset of *ssooidc.Client used by Provider.
type oidcAPI interface {
	RegisterClient(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorization(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateToken(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error)
}

// ssoAPI is the subset of *sso.Client used by Provider.
type ssoAPI interface {
	GetRoleCredentials(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error)
}

// Config holds AWS IAM Identity Center settings.
type Config struct {
	StartURL  string // e.g. "https://myorg.awsapps.com/start"
	Region    string // AWS region for the SSO service
	AccountID string // optional: shown in auth status + used for role credential fetch
	RoleName  string // optional: if set, auto-fetch role credentials after login

	// Injectable for testing; real clients are built from default AWS config when nil.
	OIDCClient   oidcAPI
	SSOClient    ssoAPI
	PollInterval time.Duration // overrides server interval; use time.Millisecond in tests
}

// Provider implements auth.Provider for AWS IAM Identity Center.
type Provider struct {
	cfg Config
}

// New returns an AWS SSO provider.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string { return name }

func (p *Provider) Login(ctx context.Context) (*auth.Session, error) {
	oidcClient, err := p.oidcClient(ctx)
	if err != nil {
		return nil, err
	}

	reg, err := oidcClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(clientName),
		ClientType: aws.String(clientType),
	})
	if err != nil {
		return nil, fmt.Errorf("register SSO client: %w", err)
	}

	da, err := oidcClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     reg.ClientId,
		ClientSecret: reg.ClientSecret,
		StartUrl:     aws.String(p.cfg.StartURL),
	})
	if err != nil {
		return nil, fmt.Errorf("start device authorization: %w", err)
	}

	// Print browser URL and code.
	verifyURL := aws.ToString(da.VerificationUriComplete)
	if verifyURL == "" {
		verifyURL = aws.ToString(da.VerificationUri)
	}
	fmt.Fprintf(os.Stderr, "\nOpen the following URL in your browser:\n\n  %s\n\n", verifyURL)
	if aws.ToString(da.VerificationUriComplete) == "" {
		fmt.Fprintf(os.Stderr, "Enter code: %s\n\n", aws.ToString(da.UserCode))
	}
	fmt.Fprintln(os.Stderr, "Waiting for authentication...")

	accessToken, expiresIn, err := p.pollForToken(ctx, oidcClient, reg, da)
	if err != nil {
		return nil, err
	}

	var expiresAt time.Time
	if expiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	} else {
		expiresAt = time.Now().Add(sessionTTL)
	}

	s := &auth.Session{
		Provider:    name,
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
		Username:    p.username(),
	}

	if p.cfg.RoleName != "" && p.cfg.AccountID != "" {
		if creds, err := p.fetchRoleCredentials(ctx, accessToken); err == nil {
			s.Extra = map[string]string{
				"aws_access_key_id":     aws.ToString(creds.AccessKeyId),
				"aws_secret_access_key": aws.ToString(creds.SecretAccessKey),
				"aws_session_token":     aws.ToString(creds.SessionToken),
			}
		}
	}

	return s, nil
}

// Refresh re-runs the device flow; AWS SSO has no refresh token.
func (p *Provider) Refresh(ctx context.Context, _ *auth.Session) (*auth.Session, error) {
	return p.Login(ctx)
}

func (p *Provider) Revoke(_ context.Context, _ *auth.Session) error {
	return nil
}

func (p *Provider) pollForToken(
	ctx context.Context,
	client oidcAPI,
	reg *ssooidc.RegisterClientOutput,
	da *ssooidc.StartDeviceAuthorizationOutput,
) (accessToken string, expiresIn int32, err error) {
	interval := p.startInterval(da.Interval)

	for {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-time.After(interval):
		}

		out, err := client.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     reg.ClientId,
			ClientSecret: reg.ClientSecret,
			GrantType:    aws.String(grantType),
			DeviceCode:   da.DeviceCode,
		})
		if err == nil {
			return aws.ToString(out.AccessToken), out.ExpiresIn, nil
		}

		var pending *ssooidctypes.AuthorizationPendingException
		var slowDown *ssooidctypes.SlowDownException
		var expired *ssooidctypes.ExpiredTokenException
		var denied *ssooidctypes.AccessDeniedException

		switch {
		case errors.As(err, &pending):
			// continue polling
		case errors.As(err, &slowDown):
			// only increase interval when not overridden (i.e. in real usage)
			if p.cfg.PollInterval == 0 {
				interval += 5 * time.Second
			}
		case errors.As(err, &expired):
			return "", 0, fmt.Errorf("device code expired; run 'devctl auth login' again")
		case errors.As(err, &denied):
			return "", 0, fmt.Errorf("access denied by user")
		default:
			return "", 0, fmt.Errorf("poll SSO token: %w", err)
		}
	}
}

func (p *Provider) fetchRoleCredentials(ctx context.Context, accessToken string) (*ssotypes.RoleCredentials, error) {
	client, err := p.getSSOClient(ctx)
	if err != nil {
		return nil, err
	}
	out, err := client.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: aws.String(accessToken),
		AccountId:   aws.String(p.cfg.AccountID),
		RoleName:    aws.String(p.cfg.RoleName),
	})
	if err != nil {
		return nil, err
	}
	return out.RoleCredentials, nil
}

func (p *Provider) oidcClient(ctx context.Context) (oidcAPI, error) {
	if p.cfg.OIDCClient != nil {
		return p.cfg.OIDCClient, nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return ssooidc.NewFromConfig(cfg), nil
}

func (p *Provider) getSSOClient(ctx context.Context) (ssoAPI, error) {
	if p.cfg.SSOClient != nil {
		return p.cfg.SSOClient, nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return sso.NewFromConfig(cfg), nil
}

func (p *Provider) startInterval(serverInterval int32) time.Duration {
	if p.cfg.PollInterval > 0 {
		return p.cfg.PollInterval
	}
	if serverInterval > 0 {
		return time.Duration(serverInterval) * time.Second
	}
	return defaultInterval
}

func (p *Provider) username() string {
	if p.cfg.AccountID != "" {
		return "sso-user@" + p.cfg.AccountID
	}
	return "sso-user"
}
