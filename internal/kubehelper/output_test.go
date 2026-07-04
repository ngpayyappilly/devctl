package kubehelper

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"devctl/pkg/output"
)

func TestPodList_JSONOutput(t *testing.T) {
	pods := PodList{
		{Name: "api-pod-abc", Status: "Running"},
		{Name: "worker-xyz", Status: "Pending"},
	}
	var buf bytes.Buffer
	require.NoError(t, output.New("json").Print(&buf, pods))

	var result []map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "api-pod-abc", result[0]["name"])
	assert.Equal(t, "Running", result[0]["status"])
}

func TestPodList_YAMLOutput(t *testing.T) {
	pods := PodList{{Name: "my-pod", Status: "Running"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("yaml").Print(&buf, pods))
	assert.Contains(t, buf.String(), "name: my-pod")
	assert.Contains(t, buf.String(), "status: Running")
}

func TestPodList_TableOutput(t *testing.T) {
	pods := PodList{{Name: "api-pod", Status: "Running"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("table").Print(&buf, pods))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Contains(t, lines[0], "NAME")
	assert.Contains(t, lines[0], "STATUS")
	assert.Contains(t, lines[1], "api-pod")
	assert.Contains(t, lines[1], "Running")
}

func TestContextResult_JSONOutput(t *testing.T) {
	ctx := ContextResult{Context: "prod-cluster"}
	var buf bytes.Buffer
	require.NoError(t, output.New("json").Print(&buf, ctx))

	var result map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "prod-cluster", result["context"])
}

func TestContextResult_YAMLOutput(t *testing.T) {
	ctx := ContextResult{Context: "staging-cluster"}
	var buf bytes.Buffer
	require.NoError(t, output.New("yaml").Print(&buf, ctx))
	assert.Contains(t, buf.String(), "context: staging-cluster")
}
