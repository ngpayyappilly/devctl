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

func TestInit_DevctlNamespaceEnvVar(t *testing.T) {
	resetViper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("defaults:\n  kube_namespace: \"\"\n"), 0o644))

	t.Setenv("DEVCTL_NAMESPACE", "production")

	require.NoError(t, config.Init(cfgPath))

	assert.Equal(t, "production", config.GetString(config.KeyKubeNamespace, ""))
}

func TestApplyProfile_LoadsValues(t *testing.T) {
	resetViper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
profiles:
  dev:
    aws_region: us-west-2
    kube_namespace: dev-ns
    kube_context: dev-cluster
`), 0o644))
	require.NoError(t, config.Init(cfgPath))

	require.NoError(t, config.ApplyProfile("dev"))

	assert.Equal(t, "us-west-2", config.GetString(config.KeyAWSRegion, ""))
	assert.Equal(t, "dev-ns", config.GetString(config.KeyKubeNamespace, ""))
	assert.Equal(t, "dev-cluster", config.GetString(config.KeyKubeContext, ""))
	assert.Equal(t, "dev", config.ActiveProfile())
}

func TestApplyProfile_DefaultProfileFromConfig(t *testing.T) {
	resetViper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
default_profile: staging
profiles:
  staging:
    aws_region: ap-southeast-1
`), 0o644))
	require.NoError(t, config.Init(cfgPath))

	// ApplyProfile("") must pick up default_profile from the config file.
	require.NoError(t, config.ApplyProfile(""))

	assert.Equal(t, "ap-southeast-1", config.GetString(config.KeyAWSRegion, ""))
	assert.Equal(t, "staging", config.ActiveProfile())
}

func TestApplyProfile_MissingProfileError(t *testing.T) {
	resetViper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
profiles:
  dev:
    aws_region: us-east-1
`), 0o644))
	require.NoError(t, config.Init(cfgPath))

	err := config.ApplyProfile("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "dev") // lists available profiles
}

func TestApplyProfile_NoProfilesDefinedError(t *testing.T) {
	resetViper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("defaults:\n  aws_region: \"\"\n"), 0o644))
	require.NoError(t, config.Init(cfgPath))

	err := config.ApplyProfile("dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no profiles are defined")
}

func TestApplyProfile_EnvVarWinsOverProfile(t *testing.T) {
	resetViper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
profiles:
  dev:
    aws_region: us-east-1
`), 0o644))
	require.NoError(t, config.Init(cfgPath))

	t.Setenv("AWS_REGION", "eu-central-1")

	require.NoError(t, config.ApplyProfile("dev"))

	// env var wins — profile's us-east-1 must not overwrite it.
	assert.Equal(t, "eu-central-1", config.GetString(config.KeyAWSRegion, ""))
}

func TestApplyProfile_EmptyExplicitAndNoDefault_IsNoop(t *testing.T) {
	resetViper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("defaults:\n  aws_region: eu-west-1\n"), 0o644))
	require.NoError(t, config.Init(cfgPath))

	// ApplyProfile("") with no default_profile must be a no-op and not error.
	require.NoError(t, config.ApplyProfile(""))
	assert.Equal(t, "", config.ActiveProfile())
	// defaults block is still readable.
	assert.Equal(t, "eu-west-1", config.GetString(config.KeyAWSRegion, ""))
}

func TestAvailableProfiles_ReturnsSortedNames(t *testing.T) {
	resetViper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
profiles:
  prod:
    aws_region: eu-west-1
  dev:
    aws_region: us-east-1
  staging:
    aws_region: ap-southeast-1
`), 0o644))
	require.NoError(t, config.Init(cfgPath))

	assert.Equal(t, []string{"dev", "prod", "staging"}, config.AvailableProfiles())
}

func TestGetString_FallbackWhenEmpty(t *testing.T) {
	resetViper()

	// No config loaded — GetString must return the fallback.
	assert.Equal(t, "us-east-1", config.GetString(config.KeyAWSRegion, "us-east-1"))
	assert.Equal(t, "default", config.GetString(config.KeyKubeNamespace, "default"))
	assert.Equal(t, "ec2-user", config.GetString(config.KeySSHUsername, "ec2-user"))
}
