package apikey_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/apikey"
	"devctl/pkg/config"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetViper() { viper.Reset() }

func TestLogin_FlagTokenWins(t *testing.T) {
	t.Setenv("DEVCTL_TOKEN", "env-token")

	p := apikey.New("flag-token")
	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "flag-token", s.AccessToken)
}

func TestLogin_EnvVarOverridesConfig(t *testing.T) {
	resetViper()
	t.Setenv("DEVCTL_TOKEN", "env-token")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("auth:\n  token: config-token\n"), 0o644))
	require.NoError(t, config.Init(cfgPath))

	p := apikey.New("")
	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "env-token", s.AccessToken)
}

func TestLogin_ConfigTokenUsedWhenNoFlag(t *testing.T) {
	resetViper()
	os.Unsetenv("DEVCTL_TOKEN")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("auth:\n  token: config-token\n"), 0o644))
	require.NoError(t, config.Init(cfgPath))

	p := apikey.New("")
	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "config-token", s.AccessToken)
}

func TestLogin_NoTokenReturnsError(t *testing.T) {
	resetViper()
	os.Unsetenv("DEVCTL_TOKEN")
	require.NoError(t, config.Init(""))

	p := apikey.New("")
	_, err := p.Login(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no API key found")
}

func TestLogin_SessionHasCorrectFields(t *testing.T) {
	p := apikey.New("my-token")
	s, err := p.Login(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "apikey", s.Provider)
	assert.Equal(t, "my-token", s.AccessToken)
	assert.Equal(t, "service-account", s.Username)
	assert.True(t, s.ExpiresAt.IsZero(), "apikey tokens should never expire")
	assert.False(t, s.IsExpired())
}

func TestRefresh_NotSupported(t *testing.T) {
	p := apikey.New("tok")
	_, err := p.Refresh(context.Background(), &auth.Session{})
	assert.ErrorIs(t, err, auth.ErrRefreshNotSupported)
}

func TestRevoke_IsNoOp(t *testing.T) {
	p := apikey.New("tok")
	assert.NoError(t, p.Revoke(context.Background(), &auth.Session{}))
}

func TestName(t *testing.T) {
	assert.Equal(t, "apikey", apikey.New("").Name())
}
