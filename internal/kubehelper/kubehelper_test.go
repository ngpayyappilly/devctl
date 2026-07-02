package kubehelper

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

func TestRestartDeploymentCmd_DryRun(t *testing.T) {
	var out bytes.Buffer
	root := newTestRoot()
	root.SetOut(&out)
	root.AddCommand(restartDeploymentCmd())
	root.SetArgs([]string{"restart", "my-api", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err, "dry-run must exit 0")
	assert.Contains(t, out.String(), "[dry-run]")
	assert.Contains(t, out.String(), "my-api")
}

func TestRestartDeploymentCmd_DryRun_NoKubectlCallMade(t *testing.T) {
	// Without --dry-run this calls kubectl which is not present in CI.
	// With --dry-run it must succeed without invoking kubectl.
	root := newTestRoot()
	root.AddCommand(restartDeploymentCmd())
	root.SetArgs([]string{"restart", "my-api", "--dry-run"})

	err := root.Execute()
	require.NoError(t, err)
}
