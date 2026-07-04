package plugincmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"devctl/internal/plugin"
	"devctl/pkg/output"
)

// NewPluginCmd returns the "devctl plugin" subcommand group.
func NewPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage and list devctl plugins",
	}
	cmd.AddCommand(listCmd())
	return cmd
}

// pluginItem is the display model for a single discovered plugin.
type pluginItem struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path" yaml:"path"`
}

// pluginList is a slice of pluginItem that implements output.Tabler.
type pluginList []pluginItem

func (pl pluginList) Headers() []string { return []string{"NAME", "PATH"} }
func (pl pluginList) Rows() [][]string {
	rows := make([][]string, len(pl))
	for i, p := range pl {
		rows[i] = []string{p.Name, p.Path}
	}
	return rows
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all discovered devctl-* plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := plugin.Discover()
			if err != nil {
				return fmt.Errorf("discover plugins: %w", err)
			}

			if len(plugins) == 0 {
				fmt.Fprintln(os.Stderr, "No plugins found. Place devctl-<name> executables on PATH or DEVCTL_PLUGIN_PATH.")
				return nil
			}

			items := make(pluginList, len(plugins))
			for i, p := range plugins {
				items[i] = pluginItem{Name: "devctl-" + p.Name, Path: p.Path}
			}
			return output.New(output.FormatFromCmd(cmd)).Print(cmd.OutOrStdout(), items)
		},
	}
}
