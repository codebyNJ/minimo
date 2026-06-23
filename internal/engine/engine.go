package engine

import (
	"github.com/codebyNJ/minimo/internal/config"
	"github.com/codebyNJ/minimo/internal/provider"
)

type Engine struct {
	providers []provider.Provider
	Store     *StateStore
}

func New(cfg config.Config) *Engine {
	return &Engine{
		providers: filterEnabled(provider.All(), cfg.EnabledProviders),
		Store:     NewStateStore(),
	}
}

func filterEnabled(all []provider.Provider, enabled []string) []provider.Provider {
	if len(enabled) == 0 {
		return all
	}
	allowed := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		allowed[name] = true
	}
	var out []provider.Provider
	for _, p := range all {
		if allowed[p.Name()] {
			out = append(out, p)
		}
	}
	return out
}

func (e *Engine) Refresh() error {
	for _, p := range e.providers {
		if !p.Detect() {
			continue
		}
		sessions, err := p.ListSessions()
		if err != nil {
			continue
		}
		for _, s := range sessions {
			ctx, err := p.ReadContext(s.ID)
			if err != nil {
				continue
			}
			e.Store.Put(s.ID, *ctx)
		}
	}
	return nil
}

type ProviderStatus struct {
	Name        string
	Detected    bool
	CheckedPath string
}

// ProviderStatuses reports detection status for every registered
// provider (the full global registry, not filtered by enabled_providers)
// so the TUI can show which harnesses ctx found on this machine.
func ProviderStatuses() []ProviderStatus {
	var out []ProviderStatus
	for _, p := range provider.All() {
		status := ProviderStatus{Name: p.Name(), Detected: p.Detect()}
		if pr, ok := p.(provider.PathReporter); ok {
			status.CheckedPath = pr.CheckedPath()
		}
		out = append(out, status)
	}
	return out
}
