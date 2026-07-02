package awssso_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/awssso"
)

// mockOIDCClient records calls and returns scripted responses.
type mockOIDCClient struct {
	registerOut  *ssooidc.RegisterClientOutput
	registerErr  error
	authorizeOut *ssooidc.StartDeviceAuthorizationOutput
	authorizeErr error
	// tokenResponses is consumed in order; last entry is repeated.
	tokenResponses []struct {
		out *ssooidc.CreateTokenOutput
		err error
	}
	tokenCalls int
}

func (m *mockOIDCClient) RegisterClient(_ context.Context, _ *ssooidc.RegisterClientInput, _ ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	return m.registerOut, m.registerErr
}

func (m *mockOIDCClient) StartDeviceAuthorization(_ context.Context, _ *ssooidc.StartDeviceAuthorizationInput, _ ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	return m.authorizeOut, m.authorizeErr
}

func (m *mockOIDCClient) CreateToken(_ context.Context, _ *ssooidc.CreateTokenInput, _ ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
	if len(m.tokenResponses) == 0 {
		return nil, errors.New("no token response configured")
	}
	idx := m.tokenCalls
	if idx >= len(m.tokenResponses) {
		idx = len(m.tokenResponses) - 1
	}
	m.tokenCalls++
	r := m.tokenResponses[idx]
	return r.out, r.err
}

type mockSSOClient struct {
	out *sso.GetRoleCredentialsOutput
	err error
}

func (m *mockSSOClient) GetRoleCredentials(_ context.Context, _ *sso.GetRoleCredentialsInput, _ ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error) {
	return m.out, m.err
}

// happyOIDCClient returns a fully-working mock for the common success path.
func happyOIDCClient() *mockOIDCClient {
	return &mockOIDCClient{
		registerOut: &ssooidc.RegisterClientOutput{
			ClientId:     aws.String("client-id"),
			ClientSecret: aws.String("client-secret"),
		},
		authorizeOut: &ssooidc.StartDeviceAuthorizationOutput{
			DeviceCode:              aws.String("device-code"),
			UserCode:                aws.String("ABCD-1234"),
			VerificationUri:         aws.String("https://device.sso.us-east-1.amazonaws.com"),
			VerificationUriComplete: aws.String("https://device.sso.us-east-1.amazonaws.com?user_code=ABCD-1234"),
			ExpiresIn:               600,
			Interval:                0,
		},
		tokenResponses: []struct {
			out *ssooidc.CreateTokenOutput
			err error
		}{
			{out: &ssooidc.CreateTokenOutput{AccessToken: aws.String("sso-access-token"), ExpiresIn: 28800}},
		},
	}
}

func TestName(t *testing.T) {
	p := awssso.New(awssso.Config{StartURL: "https://myorg.awsapps.com/start", Region: "us-east-1"})
	assert.Equal(t, "aws-sso", p.Name())
}

func TestLogin_Success(t *testing.T) {
	oidcClient := happyOIDCClient()
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		OIDCClient:   oidcClient,
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "aws-sso", s.Provider)
	assert.Equal(t, "sso-access-token", s.AccessToken)
	assert.False(t, s.IsExpired())
	assert.WithinDuration(t, time.Now().Add(8*time.Hour), s.ExpiresAt, 5*time.Second)
	assert.Equal(t, "sso-user", s.Username)
}

func TestLogin_UsernameIncludesAccountID(t *testing.T) {
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		AccountID:    "123456789012",
		OIDCClient:   happyOIDCClient(),
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "sso-user@123456789012", s.Username)
}

func TestLogin_AuthorizationPending(t *testing.T) {
	oidcClient := happyOIDCClient()
	oidcClient.tokenResponses = []struct {
		out *ssooidc.CreateTokenOutput
		err error
	}{
		{err: &ssooidctypes.AuthorizationPendingException{Message: aws.String("pending")}},
		{err: &ssooidctypes.AuthorizationPendingException{Message: aws.String("pending")}},
		{out: &ssooidc.CreateTokenOutput{AccessToken: aws.String("tok"), ExpiresIn: 28800}},
	}
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		OIDCClient:   oidcClient,
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok", s.AccessToken)
	assert.Equal(t, 3, oidcClient.tokenCalls)
}

func TestLogin_SlowDown(t *testing.T) {
	oidcClient := happyOIDCClient()
	oidcClient.tokenResponses = []struct {
		out *ssooidc.CreateTokenOutput
		err error
	}{
		{err: &ssooidctypes.SlowDownException{Message: aws.String("slow down")}},
		{out: &ssooidc.CreateTokenOutput{AccessToken: aws.String("tok"), ExpiresIn: 28800}},
	}
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		OIDCClient:   oidcClient,
		PollInterval: time.Millisecond, // prevents 5s back-off in test
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok", s.AccessToken)
}

func TestLogin_ExpiredToken(t *testing.T) {
	oidcClient := happyOIDCClient()
	oidcClient.tokenResponses = []struct {
		out *ssooidc.CreateTokenOutput
		err error
	}{
		{err: &ssooidctypes.ExpiredTokenException{Message: aws.String("expired")}},
	}
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		OIDCClient:   oidcClient,
		PollInterval: time.Millisecond,
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestLogin_AccessDenied(t *testing.T) {
	oidcClient := happyOIDCClient()
	oidcClient.tokenResponses = []struct {
		out *ssooidc.CreateTokenOutput
		err error
	}{
		{err: &ssooidctypes.AccessDeniedException{Message: aws.String("denied")}},
	}
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		OIDCClient:   oidcClient,
		PollInterval: time.Millisecond,
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestLogin_RegisterError(t *testing.T) {
	oidcClient := &mockOIDCClient{
		registerErr: errors.New("network unreachable"),
	}
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		OIDCClient:   oidcClient,
		PollInterval: time.Millisecond,
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "register SSO client")
}

func TestLogin_WithRoleCredentials(t *testing.T) {
	ssoClient := &mockSSOClient{
		out: &sso.GetRoleCredentialsOutput{
			RoleCredentials: &ssotypes.RoleCredentials{
				AccessKeyId:     aws.String("AKID"),
				SecretAccessKey: aws.String("SECRET"),
				SessionToken:    aws.String("TOKEN"),
			},
		},
	}
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		AccountID:    "123456789012",
		RoleName:     "DevAccess",
		OIDCClient:   happyOIDCClient(),
		SSOClient:    ssoClient,
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AKID", s.Extra["aws_access_key_id"])
	assert.Equal(t, "SECRET", s.Extra["aws_secret_access_key"])
	assert.Equal(t, "TOKEN", s.Extra["aws_session_token"])
}

func TestRefresh_RerunsDeviceFlow(t *testing.T) {
	p := awssso.New(awssso.Config{
		StartURL:     "https://myorg.awsapps.com/start",
		Region:       "us-east-1",
		OIDCClient:   happyOIDCClient(),
		PollInterval: time.Millisecond,
	})

	existing := &auth.Session{Provider: "aws-sso", AccessToken: "old-token"}
	s, err := p.Refresh(context.Background(), existing)
	require.NoError(t, err)
	assert.Equal(t, "aws-sso", s.Provider)
	assert.Equal(t, "sso-access-token", s.AccessToken)
}

func TestRevoke_IsNoOp(t *testing.T) {
	p := awssso.New(awssso.Config{StartURL: "https://myorg.awsapps.com/start", Region: "us-east-1"})
	assert.NoError(t, p.Revoke(context.Background(), &auth.Session{}))
}
