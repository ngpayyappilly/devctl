package config_test

import (
	"devctl/pkg/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetViper() {
	viper.Reset()
}

func TestInit_LoadsFromFile(t *testing.T) {
	resetViper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
defaults:
  aws_region: eu-west-1
  kube_namespace: staging
  ssh_username: ubuntu
`), 0o644)
	require.NoError(t, err)

	require.NoError(t, config.Init(cfgPath))

	assert.Equal(t, "eu-west-1", config.GetString(config.KeyAWSRegion, "us-east-1"))
	assert.Equal(t, "staging", config.GetString(config.KeyKubeNamespace, "default"))
	assert.Equal(t, "ubuntu", config.GetString(config.KeySSHUsername, "ec2-user"))
}

func TestInit_EnvVarOverridesFile(t *testing.T) {
	resetViper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
defaults:
  aws_region: eu-west-1
`), 0o644)
	require.NoError(t, err)

	t.Setenv("AWS_REGION", "ap-southeast-1")

	require.NoError(t, config.Init(cfgPath))

	// AWS_REGION env var must win over the file value.
	assert.Equal(t, "ap-southeast-1", config.GetString(config.KeyAWSRegion, "us-east-1"))
}

func TestInit_MissingFileIsGraceful(t *testing.T) {
	resetViper()

	// Point at a path that does not exist — must not return an error.
	err := config.Init("/tmp/devctl-does-not-exist-config.yaml")
	assert.NoError(t, err)
}

func TestInit_MalformedFileWarnsAndContinues(t *testing.T) {
	resetViper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	// Write invalid YAML.
	err := os.WriteFile(cfgPath, []byte(":: not valid yaml ::"), 0o644)
	require.NoError(t, err)

	// Must not return an error (just warns on stderr).
	assert.NoError(t, config.Init(cfgPath))
}

func TestGetString_FallbackWhenEmpty(t *testing.T) {
	resetViper()

	// No config loaded — GetString must return the fallback.
	assert.Equal(t, "us-east-1", config.GetString(config.KeyAWSRegion, "us-east-1"))
	assert.Equal(t, "default", config.GetString(config.KeyKubeNamespace, "default"))
	assert.Equal(t, "ec2-user", config.GetString(config.KeySSHUsername, "ec2-user"))
}
