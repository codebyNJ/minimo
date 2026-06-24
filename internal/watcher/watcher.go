package watcher

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/codebyNJ/minimo/internal/logging"
)

type Watcher struct {
	fsw      *fsnotify.Watcher
	debounce time.Duration

	mu     sync.Mutex
	timers map[string]*time.Timer

	Events chan string
}

func New(debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fsw:      fsw,
		debounce: debounce,
		timers:   make(map[string]*time.Timer),
		Events:   make(chan string, 64),
	}, nil
}

func (w *Watcher) AddRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return w.fsw.Add(path)
	})
}

func (w *Watcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if ev.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			if err := w.fsw.Add(ev.Name); err != nil {
				// New subdirectory won't be watched; sessions created under it
				// surface on the next poll tick instead of immediately.
				logging.Debugf("watcher: add %s failed: %v", ev.Name, err)
			}
			return
		}
	}
	if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if t, ok := w.timers[ev.Name]; ok {
		t.Stop()
	}
	path := ev.Name
	// Delete the entry when the timer fires; otherwise w.timers accumulates one
	// dead entry per distinct path touched and grows unbounded over a long
	// session monitoring hundreds of JSONL files.
	w.timers[path] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.timers, path)
		w.mu.Unlock()
		w.Events <- path
	})
}

// pendingTimers reports how many debounce timers are currently tracked. Used
// by tests to assert the map does not leak after timers fire.
func (w *Watcher) pendingTimers() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.timers)
}

func (w *Watcher) Close() error {
	return w.fsw.Close()
}
