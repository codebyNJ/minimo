package engine

import (
	"sync"

	"github.com/codebyNJ/minimo/internal/provider"
)

type StateStore struct {
	mu    sync.RWMutex
	items map[string]provider.SessionContext
}

func NewStateStore() *StateStore {
	return &StateStore{items: make(map[string]provider.SessionContext)}
}

func (s *StateStore) Put(sessionID string, ctx provider.SessionContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[sessionID] = ctx
}

func (s *StateStore) Get(sessionID string) (provider.SessionContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.items[sessionID]
	return c, ok
}

func (s *StateStore) All() []provider.SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]provider.SessionContext, 0, len(s.items))
	for _, c := range s.items {
		out = append(out, c)
	}
	return out
}

// Retain drops sessions that are gone. An entry is removed only when its
// provider was successfully enumerated this cycle (its name is in
// enumeratedProviders) yet the session id was not seen — i.e. its transcript
// or DB row is genuinely gone. Sessions from a provider that failed or was not
// detected this cycle are kept, so a transient read error doesn't flicker the
// whole list. This caps the store at live on-disk sessions instead of growing
// for every session ever seen.
func (s *StateStore) Retain(seen, enumeratedProviders map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range s.items {
		if !seen[id] && enumeratedProviders[c.Session.Provider] {
			delete(s.items, id)
		}
	}
}
