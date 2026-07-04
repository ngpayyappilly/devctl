package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"devctl/pkg/output"
)

// simpleList is a minimal Tabler for testing.
type simpleList []struct {
	Name string `json:"name" yaml:"name"`
}

func (s simpleList) Headers() []string { return []string{"NAME"} }
func (s simpleList) Rows() [][]string {
	rows := make([][]string, len(s))
	for i, item := range s {
		rows[i] = []string{item.Name}
	}
	return rows
}

func TestJSONPrinter_ProducesValidJSON(t *testing.T) {
	data := simpleList{{Name: "alpha"}, {Name: "beta"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("json").Print(&buf, data))

	var result []map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "alpha", result[0]["name"])
	assert.Equal(t, "beta", result[1]["name"])
}

func TestYAMLPrinter_ProducesValidYAML(t *testing.T) {
	data := simpleList{{Name: "alpha"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("yaml").Print(&buf, data))
	assert.Contains(t, buf.String(), "name: alpha")
}

func TestTablePrinter_RendersHeadersAndRows(t *testing.T) {
	data := simpleList{{Name: "alpha"}, {Name: "beta"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("table").Print(&buf, data))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, "NAME", strings.TrimSpace(lines[0]))
	assert.Contains(t, lines[1], "alpha")
	assert.Contains(t, lines[2], "beta")
}

func TestTablePrinter_NonTablerFallsBackToJSON(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, output.New("table").Print(&buf, map[string]string{"key": "val"}))
	assert.Contains(t, buf.String(), `"key"`)
}

func TestNew_UnknownFormatDefaultsToTable(t *testing.T) {
	data := simpleList{{Name: "x"}}
	var buf bytes.Buffer
	require.NoError(t, output.New("bogus").Print(&buf, data))
	assert.Contains(t, buf.String(), "NAME")
}
