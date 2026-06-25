package engine

import (
	"github.com/codebyNJ/minimo/internal/config"
	"github.com/codebyNJ/minimo/internal/logging"
	"github.com/codebyNJ/minimo/internal/pricing"
	"github.com/codebyNJ/minimo/internal/provider"
)

type Engine struct {
	providers []provider.Provider
	catalog   pricing.Catalog
	Store     *StateStore
}

func New(cfg config.Config, cat pricing.Catalog) *Engine {
	return &Engine{
		providers: filterEnabled(provider.All(), cfg.EnabledProviders),
		catalog:   cat,
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
			// Whole provider dropped this cycle — surface it for `--debug`
			// users rather than silently showing an empty list.
			logging.Errorf("%s: list sessions failed: %v", p.Name(), err)
			continue
		}
		for _, s := range sessions {
			ctx, err := p.ReadContext(s.ID)
			if err != nil {
				logging.Debugf("%s: read context for session %s failed: %v", p.Name(), s.ID, err)
				continue
			}
			if cost, changed := estimatedCost(e.catalog, *ctx); changed {
				ctx.Cost = cost
			}
			backfillContextLimit(e.catalog, ctx)
			e.Store.Put(s.ID, *ctx)
		}
	}
	return nil
}

// estimatedCost returns an estimated Cost (and changed=true) when the session
// has no exact cost but its model is in the pricing catalog. Exact costs are
// left untouched.
func estimatedCost(cat pricing.Catalog, c provider.SessionContext) (provider.Cost, bool) {
	if c.Cost.Known {
		return c.Cost, false
	}
	if usd, ok := cat.Estimate(c.Session.Model, c.Tokens); ok {
		return provider.Cost{USD: usd, Known: true, Source: provider.CostSourceEstimated}, true
	}
	return c.Cost, false
}

// backfillContextLimit fills a missing context-window denominator from the
// pricing catalog, so models a provider didn't hardcode a window for can
// still show a percentage bar. It only acts when the provider reported known
// context usage with no limit and the catalog actually carries a window —
// never inventing a denominator (a 0 limit stays 0, rendering a raw count).
func backfillContextLimit(cat pricing.Catalog, c *provider.SessionContext) {
	if !c.Context.Known || c.Context.Limit > 0 {
		return
	}
	if win, ok := cat.ContextWindow(c.Session.Model); ok {
		c.Context.Limit = win
	}
}

type ProviderStatus struct {
	Name        string
	Detected    bool
	CheckedPath string
	Plan        provider.PlanInfo
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
		if pl, ok := p.(provider.PlanReporter); ok {
			status.Plan = pl.Plan()
		}
		out = append(out, status)
	}
	return out
}
