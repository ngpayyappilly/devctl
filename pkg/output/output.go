package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// Format identifies a supported output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// Tabler is implemented by result types that know how to render as a text table.
type Tabler interface {
	Headers() []string
	Rows() [][]string
}

// Printer writes a structured value to an io.Writer.
type Printer interface {
	Print(w io.Writer, v any) error
}

// New returns a Printer for the given format string.
// Unrecognised values fall back to table.
func New(format string) Printer {
	switch Format(format) {
	case FormatJSON:
		return &jsonPrinter{}
	case FormatYAML:
		return &yamlPrinter{}
	default:
		return &tablePrinter{}
	}
}

// FormatFromCmd reads the --output persistent flag from cmd's root, defaulting to "table".
func FormatFromCmd(cmd *cobra.Command) string {
	if f, err := cmd.Root().PersistentFlags().GetString("output"); err == nil && f != "" {
		return f
	}
	return string(FormatTable)
}

// --- JSON ---

type jsonPrinter struct{}

func (p *jsonPrinter) Print(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// --- YAML ---

type yamlPrinter struct{}

func (p *yamlPrinter) Print(w io.Writer, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// --- Table ---

type tablePrinter struct{}

func (p *tablePrinter) Print(w io.Writer, v any) error {
	t, ok := v.(Tabler)
	if !ok {
		return json.NewEncoder(w).Encode(v)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headers := t.Headers()
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)
	for _, row := range t.Rows() {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, cell)
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}
