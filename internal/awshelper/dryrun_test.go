package awshelper

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRoot returns a minimal root command that carries the global --dry-run
// persistent flag, mirroring what cmd/devctl/main.go does at runtime.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "devctl", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().BoolP("dry-run", "n", false, "dry run")
	return root
}

func TestDeleteStackCmd_DryRun(t *testing.T) {
	var out bytes.Buffer
	root := newTestRoot()
	root.SetOut(&out)
	root.AddCommand(deleteStackCmd())
	root.SetArgs([]string{"delete-cf-stack", "my-stack", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err, "dry-run must exit 0")
	assert.Contains(t, out.String(), "[dry-run]")
	assert.Contains(t, out.String(), "my-stack")
}

func TestDeleteStackCmd_DryRun_NoAWSCallMade(t *testing.T) {
	// Without --dry-run the command would attempt an AWS call and fail in CI
	// (no credentials). With --dry-run it must succeed even without creds.
	root := newTestRoot()
	root.AddCommand(deleteStackCmd())
	root.SetArgs([]string{"delete-cf-stack", "prod-stack", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err)
}

func TestSSHEC2Cmd_DryRun(t *testing.T) {
	var out bytes.Buffer
	root := newTestRoot()
	root.SetOut(&out)
	root.AddCommand(sshEC2Cmd())
	root.SetArgs([]string{"ssh-ec2", "-i", "i-0abc123", "-k", "~/.ssh/id_rsa", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err, "dry-run must exit 0")
	assert.Contains(t, out.String(), "[dry-run]")
	assert.Contains(t, out.String(), "i-0abc123")
}
