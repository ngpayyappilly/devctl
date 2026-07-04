package profilecmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"devctl/pkg/config"
	"devctl/pkg/output"
)

// NewProfileCmd returns the "devctl profile" subcommand group.
func NewProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named configuration profiles",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(useCmd())
	return cmd
}

// profileItem is the display model for a single profile entry.
type profileItem struct {
	Name   string `json:"name" yaml:"name"`
	Active bool   `json:"active" yaml:"active"`
}

// profileList implements output.Tabler.
type profileList []profileItem

func (pl profileList) Headers() []string { return []string{"NAME", "ACTIVE"} }
func (pl profileList) Rows() [][]string {
	rows := make([][]string, len(pl))
	for i, p := range pl {
		active := ""
		if p.Active {
			active = "*"
		}
		rows[i] = []string{p.Name, active}
	}
	return rows
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured profiles and mark the active one",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles := config.AvailableProfiles()
			if len(profiles) == 0 {
				fmt.Fprintln(os.Stderr, "No profiles defined. Add a 'profiles:' block to your config file or run: devctl config init")
				return nil
			}
			active := config.ActiveProfile()
			items := make(profileList, len(profiles))
			for i, name := range profiles {
				items[i] = profileItem{Name: name, Active: name == active}
			}
			return output.New(output.FormatFromCmd(cmd)).Print(cmd.OutOrStdout(), items)
		},
	}
}

func useCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Set the default profile in the config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			available := config.AvailableProfiles()
			found := false
			for _, name := range available {
				if name == profileName {
					found = true
					break
				}
			}
			if !found {
				if len(available) == 0 {
					return fmt.Errorf("profile %q not found; no profiles are defined in the config file", profileName)
				}
				return fmt.Errorf("profile %q not found; available: %s", profileName, strings.Join(available, ", "))
			}

			if err := config.SetDefaultProfile(profileName); err != nil {
				return fmt.Errorf("save default profile: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default profile set to %q\n", profileName)
			return nil
		},
	}
}
