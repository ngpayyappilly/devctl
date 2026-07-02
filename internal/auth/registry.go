package auth

import (
	"fmt"
	"sort"
)

var registry = map[string]Provider{}

// Register adds a provider to the global registry. Call from each provider
// package's init() or from main.go before the CLI runs.
func Register(p Provider) {
	registry[p.Name()] = p
}

// Lookup returns the named provider or an error listing available names.
func Lookup(name string) (Provider, error) {
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown auth provider %q (available: %s)", name, availableNames())
	}
	return p, nil
}

func availableNames() string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "none registered"
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}
