package githelper

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "devctl", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().BoolP("dry-run", "n", false, "dry run")
	return root
}

func TestCommitCmd_DryRun(t *testing.T) {
	var out bytes.Buffer
	root := newTestRoot()
	root.SetOut(&out)
	root.AddCommand(commitCmd())
	root.SetArgs([]string{"commit", "-m", "fix: typo", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err, "dry-run must exit 0")
	assert.Contains(t, out.String(), "[dry-run]")
	assert.Contains(t, out.String(), "fix: typo")
}

func TestCommitCmd_DryRun_NoGitCallMade(t *testing.T) {
	// Without --dry-run this would run "git add ." and "git commit" which would
	// fail outside a clean repo. With --dry-run it must succeed.
	root := newTestRoot()
	root.AddCommand(commitCmd())
	root.SetArgs([]string{"commit", "-m", "chore: test commit", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err)
}

func TestPushCmd_DryRun(t *testing.T) {
	var out bytes.Buffer
	root := newTestRoot()
	root.SetOut(&out)
	root.AddCommand(pushCmd())
	root.SetArgs([]string{"push", "-r", "origin", "-b", "main", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err, "dry-run must exit 0")
	assert.Contains(t, out.String(), "[dry-run]")
	assert.Contains(t, out.String(), "origin")
	assert.Contains(t, out.String(), "main")
}

func TestPushCmd_DryRun_NoGitCallMade(t *testing.T) {
	root := newTestRoot()
	root.AddCommand(pushCmd())
	root.SetArgs([]string{"push", "-r", "origin", "-b", "my-branch", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err)
}
