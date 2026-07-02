package githelper

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	deverrors "devctl/pkg/errors"
)

func NewGitHelperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Perform common Git actions quickly",
	}

	cmd.AddCommand(cloneCmd())
	cmd.AddCommand(checkoutCmd())
	cmd.AddCommand(commitCmd())
	cmd.AddCommand(pushCmd())

	return cmd
}

func cloneCmd() *cobra.Command {
	var repo string
	var dir string

	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone a Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return deverrors.NewUsageError("--repo is required")
			}

			gitArgs := []string{"clone", repo}
			if dir != "" {
				gitArgs = append(gitArgs, dir)
			}
			return runGitCommand(gitArgs...)
		},
	}

	cmd.Flags().StringVarP(&repo, "repo", "r", "", "Repository URL")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Target directory (optional)")
	return cmd
}

func checkoutCmd() *cobra.Command {
	var branch string

	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Checkout a Git branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			if branch == "" {
				return deverrors.NewUsageError("--branch is required")
			}
			return runGitCommand("checkout", branch)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name")
	return cmd
}

func commitCmd() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Create a Git commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" {
				return deverrors.NewUsageError("--message is required")
			}
			if err := runGitCommand("add", "."); err != nil {
				return err
			}
			return runGitCommand("commit", "-m", message)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message")
	return cmd
}

func pushCmd() *cobra.Command {
	var remote string
	var branch string

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push changes to a Git remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			if remote == "" {
				remote = "origin"
			}
			if branch == "" {
				out, err := exec.Command("git", "branch", "--show-current").Output()
				if err != nil {
					return fmt.Errorf("determine current branch: %w", err)
				}
				branch = strings.TrimSpace(string(out))
			}
			return runGitCommand("push", remote, branch)
		},
	}

	cmd.Flags().StringVarP(&remote, "remote", "r", "origin", "Git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Git branch")
	return cmd
}

func runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Printf("▶️ git %s\n", strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}
