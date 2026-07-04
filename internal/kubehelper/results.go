package kubehelper

// Pod represents a Kubernetes pod summary.
type Pod struct {
	Name   string `json:"name" yaml:"name"`
	Status string `json:"status" yaml:"status"`
}

// PodList is a slice of Pod that satisfies output.Tabler.
type PodList []Pod

func (pl PodList) Headers() []string { return []string{"NAME", "STATUS"} }
func (pl PodList) Rows() [][]string {
	rows := make([][]string, len(pl))
	for i, p := range pl {
		rows[i] = []string{p.Name, p.Status}
	}
	return rows
}

// ContextResult holds the current Kubernetes context name.
type ContextResult struct {
	Context string `json:"context" yaml:"context"`
}

func (c ContextResult) Headers() []string { return []string{"CONTEXT"} }
func (c ContextResult) Rows() [][]string  { return [][]string{{c.Context}} }
