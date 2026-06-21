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
