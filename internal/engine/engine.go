package engine

import (
	"github.com/codebyNJ/minimo/internal/provider"
)

type Engine struct {
	providers []provider.Provider
	Store     *StateStore
}

func New() *Engine {
	return &Engine{
		providers: provider.All(),
		Store:     NewStateStore(),
	}
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
