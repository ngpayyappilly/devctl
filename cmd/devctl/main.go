package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/apikey"
	"devctl/internal/authcmd"
	"devctl/internal/awshelper"
	"devctl/internal/configcmd"
	"devctl/internal/githelper"
	"devctl/internal/kubehelper"
	"devctl/internal/netcheck"
	"devctl/pkg/config"
	deverrors "devctl/pkg/errors"
)

var (
	version   = "dev"
	gitSha    = "none"
	buildDate = "unknown"
)

func main() {
	// Register auth providers. Token resolved at login time from flag/env/config.
	auth.Register(apikey.New(""))

	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "devctl",
		Short: "Developer and SRE utility for Git, Kubernetes, AWS, and networking",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return config.Init(cfgFile)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: .devctl.yaml or ~/.devctl/config.yaml)")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the devctl version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s\nGit SHA: %s\nBuilt at: %s\n", version, gitSha, buildDate)
		},
	})
	rootCmd.AddCommand(netcheck.NewNetCheckCmd())
	rootCmd.AddCommand(kubehelper.NewKubeHelperCmd())
	rootCmd.AddCommand(awshelper.NewAwsHelperCmd())
	rootCmd.AddCommand(githelper.NewGitHelperCmd())
	rootCmd.AddCommand(configcmd.NewConfigCmd())
	rootCmd.AddCommand(authcmd.NewAuthCmd())

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(deverrors.ExitCode(err))
	}
}
