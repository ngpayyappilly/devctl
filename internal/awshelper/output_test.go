package awshelper

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"devctl/pkg/output"
)

func TestBucketList_JSONOutput(t *testing.T) {
	buckets := BucketList{{Name: "alpha"}, {Name: "beta"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("json").Print(&buf, buckets))

	var result []map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "alpha", result[0]["name"])
	assert.Equal(t, "beta", result[1]["name"])
}

func TestBucketList_YAMLOutput(t *testing.T) {
	buckets := BucketList{{Name: "my-bucket"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("yaml").Print(&buf, buckets))
	assert.Contains(t, buf.String(), "name: my-bucket")
}

func TestBucketList_TableOutput(t *testing.T) {
	buckets := BucketList{{Name: "alpha"}, {Name: "beta"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("table").Print(&buf, buckets))
	assert.Contains(t, buf.String(), "NAME")
	assert.Contains(t, buf.String(), "alpha")
	assert.Contains(t, buf.String(), "beta")
}

func TestInstanceList_JSONOutput(t *testing.T) {
	instances := InstanceList{{ID: "i-123", State: "running", Type: "t3.micro"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("json").Print(&buf, instances))

	var result []map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "i-123", result[0]["id"])
	assert.Equal(t, "running", result[0]["state"])
}

func TestInstanceList_YAMLOutput(t *testing.T) {
	instances := InstanceList{{ID: "i-abc", State: "stopped", Type: "t2.small"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("yaml").Print(&buf, instances))
	assert.Contains(t, buf.String(), "id: i-abc")
	assert.Contains(t, buf.String(), "state: stopped")
}

func TestStackList_JSONOutput(t *testing.T) {
	stacks := StackList{{Name: "my-stack", Status: "CREATE_COMPLETE"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("json").Print(&buf, stacks))

	var result []map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "my-stack", result[0]["name"])
	assert.Equal(t, "CREATE_COMPLETE", result[0]["status"])
}

func TestIAMPolicyList_TableOutput(t *testing.T) {
	policies := IAMPolicyList{{PolicyName: "AdministratorAccess"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("table").Print(&buf, policies))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Contains(t, lines[0], "POLICY NAME")
	assert.Contains(t, lines[1], "AdministratorAccess")
}
