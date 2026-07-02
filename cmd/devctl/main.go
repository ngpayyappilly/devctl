package main

import (
	"devctl/internal/awshelper"
	"devctl/internal/configcmd"
	"devctl/internal/githelper"
	"devctl/internal/kubehelper"
	"devctl/internal/netcheck"
	"devctl/pkg/config"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	gitSha    = "none"
	buildDate = "unknown"
)

func main() {
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
