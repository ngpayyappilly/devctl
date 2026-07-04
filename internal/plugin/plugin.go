package plugin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Plugin represents a discovered devctl-<name> binary on the search path.
type Plugin struct {
	Name string // short name (without "devctl-" prefix)
	Path string // absolute path to the binary
}

// Discover scans DEVCTL_PLUGIN_PATH (prepended) and then PATH for executables
// whose names match devctl-<name>. The first found binary for each name wins.
// Non-executable files are warned and skipped; unreadable directories are
// silently skipped.
func Discover() ([]Plugin, error) {
	seen := map[string]bool{}
	var plugins []Plugin

	for _, dir := range searchDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fname := entry.Name()
			if !strings.HasPrefix(fname, "devctl-") {
				continue
			}
			pluginName := strings.TrimPrefix(fname, "devctl-")
			if pluginName == "" || seen[pluginName] {
				continue
			}
			absPath := filepath.Join(dir, fname)
			if !isExecutable(absPath) {
				fmt.Fprintf(os.Stderr, "warning: plugin %s is not executable, skipping\n", absPath)
				continue
			}
			seen[pluginName] = true
			plugins = append(plugins, Plugin{Name: pluginName, Path: absPath})
		}
	}

	return plugins, nil
}

// RegisterPlugins discovers all devctl-* plugins and registers each as a
// Cobra subcommand of root. Built-in commands always win: if a plugin name
// conflicts, a warning is printed and the plugin is skipped.
func RegisterPlugins(root *cobra.Command) {
	plugins, err := Discover()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: plugin discovery: %v\n", err)
		return
	}

	builtin := map[string]bool{}
	for _, cmd := range root.Commands() {
		builtin[cmd.Name()] = true
	}

	for _, p := range plugins {
		if builtin[p.Name] {
			fmt.Fprintf(os.Stderr, "warning: plugin %q conflicts with built-in command, skipping\n", p.Name)
			continue
		}
		root.AddCommand(p.NewCmd())
	}
}

// NewCmd returns a Cobra command that execs the plugin binary with forwarded
// args and environment. Flag parsing is disabled so all tokens are passed
// verbatim to the plugin.
func (p *Plugin) NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:                p.Name,
		Short:              fmt.Sprintf("External plugin (%s)", p.Path),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command(p.Path, args...) //nolint:gosec
			c.Stdin = os.Stdin
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			if err := c.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					os.Exit(exitErr.ExitCode())
				}
				return fmt.Errorf("plugin %s: %w", p.Name, err)
			}
			return nil
		},
	}
}

// searchDirs returns the ordered list of directories to scan: DEVCTL_PLUGIN_PATH
// entries first, then the standard PATH entries.
func searchDirs() []string {
	var dirs []string
	if extra := os.Getenv("DEVCTL_PLUGIN_PATH"); extra != "" {
		dirs = append(dirs, filepath.SplitList(extra)...)
	}
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		dirs = append(dirs, filepath.SplitList(pathEnv)...)
	}
	return dirs
}

// isExecutable reports whether the file at path has at least one execute bit set.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}
