package configcmd

import (
	"devctl/pkg/config"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage devctl configuration",
	}

	cmd.AddCommand(initCmd())
	return cmd
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a default config file at ~/.devctl/config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}

			dir := filepath.Join(home, ".devctl")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create config directory: %w", err)
			}

			dest := filepath.Join(dir, "config.yaml")
			if _, err := os.Stat(dest); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "config file already exists at %s — not overwriting\n", dest)
				return nil
			}

			if err := os.WriteFile(dest, []byte(config.Template), 0o644); err != nil {
				return fmt.Errorf("write config file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", dest)
			return nil
		},
	}
}
