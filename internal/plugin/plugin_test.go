package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createExe writes a minimal shell script and marks it executable.
func createExe(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho hello\n"), 0o755))
}

func TestDiscover_FindsExecutablePlugins(t *testing.T) {
	dir := t.TempDir()
	createExe(t, filepath.Join(dir, "devctl-foo"))
	createExe(t, filepath.Join(dir, "devctl-bar"))
	// Not a plugin — wrong prefix.
	createExe(t, filepath.Join(dir, "kubectl-foo"))

	t.Setenv("DEVCTL_PLUGIN_PATH", dir)
	t.Setenv("PATH", t.TempDir()) // empty dir — no interference from real PATH

	plugins, err := Discover()
	require.NoError(t, err)

	names := pluginNames(plugins)
	assert.Contains(t, names, "foo")
	assert.Contains(t, names, "bar")
	assert.NotContains(t, names, "kubectl-foo")
}

func TestDiscover_SkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	createExe(t, filepath.Join(dir, "devctl-good"))
	// Non-executable file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "devctl-bad"), []byte("#!/bin/sh"), 0o644))

	t.Setenv("DEVCTL_PLUGIN_PATH", dir)
	t.Setenv("PATH", t.TempDir())

	plugins, err := Discover()
	require.NoError(t, err)

	names := pluginNames(plugins)
	assert.Contains(t, names, "good")
	assert.NotContains(t, names, "bad")
}

func TestDiscover_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	// A directory named devctl-subdir must be ignored.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "devctl-subdir"), 0o755))

	t.Setenv("DEVCTL_PLUGIN_PATH", dir)
	t.Setenv("PATH", t.TempDir())

	plugins, err := Discover()
	require.NoError(t, err)
	assert.Empty(t, pluginNames(plugins))
}

func TestDiscover_FirstPathWins(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	createExe(t, filepath.Join(first, "devctl-deploy"))
	createExe(t, filepath.Join(second, "devctl-deploy"))

	t.Setenv("DEVCTL_PLUGIN_PATH", first+string(os.PathListSeparator)+second)
	t.Setenv("PATH", t.TempDir())

	plugins, err := Discover()
	require.NoError(t, err)

	var found *Plugin
	for i := range plugins {
		if plugins[i].Name == "deploy" {
			found = &plugins[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, filepath.Join(first, "devctl-deploy"), found.Path)
}

func TestDiscover_DevctlPluginPathBeforePath(t *testing.T) {
	pluginDir := t.TempDir()
	pathDir := t.TempDir()
	createExe(t, filepath.Join(pluginDir, "devctl-mytool"))
	createExe(t, filepath.Join(pathDir, "devctl-mytool"))

	t.Setenv("DEVCTL_PLUGIN_PATH", pluginDir)
	t.Setenv("PATH", pathDir)

	plugins, err := Discover()
	require.NoError(t, err)

	for _, p := range plugins {
		if p.Name == "mytool" {
			assert.Equal(t, filepath.Join(pluginDir, "devctl-mytool"), p.Path,
				"DEVCTL_PLUGIN_PATH entry must win over PATH entry")
			return
		}
	}
	t.Fatal("mytool plugin not found")
}

func TestPlugin_NewCmd_Properties(t *testing.T) {
	p := Plugin{Name: "mytool", Path: "/usr/local/bin/devctl-mytool"}
	cmd := p.NewCmd()

	assert.Equal(t, "mytool", cmd.Use)
	assert.True(t, cmd.DisableFlagParsing, "flags must be forwarded verbatim")
}

// pluginNames extracts Name fields for easier assertions.
func pluginNames(plugins []Plugin) []string {
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.Name
	}
	return names
}
