package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

const debounceDuration = 500 * time.Millisecond

// EventKind categorises a file system event.
type EventKind int

const (
	EventCreate EventKind = iota
	EventWrite
	EventRemove
	EventRename
	EventChmod
)

func (k EventKind) String() string {
	switch k {
	case EventCreate:
		return "create"
	case EventWrite:
		return "write"
	case EventRemove:
		return "remove"
	case EventRename:
		return "rename"
	default:
		return "chmod"
	}
}

// FileEvent is emitted when a file system change is detected.
type FileEvent struct {
	Path string
	Kind EventKind
}

// Watcher watches one or more directory trees for changes.
type Watcher struct {
	fsw    *fsnotify.Watcher
	Events chan FileEvent
	Errors chan error

	mu      sync.Mutex
	timers  map[string]*time.Timer
}

func New() (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:    fsw,
		Events: make(chan FileEvent, 256),
		Errors: make(chan error, 16),
		timers: make(map[string]*time.Timer),
	}
	go w.loop()
	return w, nil
}

// Add adds a path (and all subdirectories recursively) to the watch list.
func (w *Watcher) Add(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if d.IsDir() {
			if err := w.fsw.Add(path); err != nil {
				log.Warn().Str("path", path).Err(err).Msg("Could not watch directory")
			}
		}
		return nil
	})
}

// Remove stops watching a path.
func (w *Watcher) Remove(path string) error {
	return w.fsw.Remove(path)
}

// Close shuts down the watcher.
func (w *Watcher) Close() error {
	return w.fsw.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			// When a new directory is created, start watching it too
			if event.Has(fsnotify.Create) {
				if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
					_ = w.Add(event.Name)
				}
			}
			w.debounce(event)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.Errors <- err
		}
	}
}

func (w *Watcher) debounce(event fsnotify.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, exists := w.timers[event.Name]; exists {
		t.Reset(debounceDuration)
		return
	}

	w.timers[event.Name] = time.AfterFunc(debounceDuration, func() {
		w.mu.Lock()
		delete(w.timers, event.Name)
		w.mu.Unlock()
		w.Events <- FileEvent{
			Path: event.Name,
			Kind: opToKind(event.Op),
		}
	})
}

func opToKind(op fsnotify.Op) EventKind {
	switch {
	case op.Has(fsnotify.Create):
		return EventCreate
	case op.Has(fsnotify.Write):
		return EventWrite
	case op.Has(fsnotify.Remove):
		return EventRemove
	case op.Has(fsnotify.Rename):
		return EventRename
	default:
		return EventChmod
	}
}
