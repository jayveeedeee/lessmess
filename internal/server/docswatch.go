package server

import (
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"lessmess/internal/docs"
)

// docsWatcher watches the covered directories with fsnotify and emits
// debounced notifications when anything in them changes. It exists so the
// explorer can live-refresh as docs (or the tree) change — e.g. after a
// gardener run. The watch set is rebuilt after every quiet period, so
// covered directories added or removed between runs are picked up.
type docsWatcher struct {
	root string
	cfg  *docs.Config
	fsw  *fsnotify.Watcher

	mu      sync.Mutex
	watched map[string]bool
	events  chan struct{}

	done   chan struct{}
	closed sync.Once
}

func newDocsWatcher(root string, cfg *docs.Config) (*docsWatcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &docsWatcher{
		root:    root,
		cfg:     cfg,
		fsw:     fsw,
		watched: map[string]bool{},
		events:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	w.resync()
	go w.loop()
	return w, nil
}

// Events returns the debounced notification channel.
func (w *docsWatcher) Events() <-chan struct{} { return w.events }

// Close stops the watcher.
func (w *docsWatcher) Close() error {
	var err error
	w.closed.Do(func() {
		close(w.done)
		err = w.fsw.Close()
	})
	return err
}

// resync rebuilds the watch set from the current covered tree.
func (w *docsWatcher) resync() {
	tree, err := docs.Walk(w.root, w.cfg)
	if err != nil {
		slog.Warn("docs watcher walk", "err", err)
		return
	}
	want := map[string]bool{}
	for _, d := range docs.PostOrder(tree) {
		want[d.Abs] = true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for abs := range w.watched {
		if !want[abs] {
			_ = w.fsw.Remove(abs)
			delete(w.watched, abs)
		}
	}
	for abs := range want {
		if !w.watched[abs] {
			if err := w.fsw.Add(abs); err != nil {
				slog.Warn("docs watcher add", "dir", abs, "err", err)
				continue
			}
			w.watched[abs] = true
		}
	}
}

// ignore filters events that should not trigger a refresh: our own atomic
// temp files and hidden entries (editor noise, .DS_Store).
func (w *docsWatcher) ignore(name string) bool {
	base := filepath.Base(name)
	return strings.HasPrefix(base, ".tt-") || strings.HasPrefix(base, ".")
}

func (w *docsWatcher) loop() {
	var timer *time.Timer
	var fire <-chan time.Time
	reset := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.NewTimer(300 * time.Millisecond)
		fire = timer.C
	}
	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			// Attribute-only events (atime updates from readers like git
			// status, or chmod) are not content changes; ignore them.
			if ev.Op&^ fsnotify.Chmod == 0 {
				continue
			}
			if w.ignore(ev.Name) {
				continue
			}
			reset()
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			slog.Warn("docs watcher error", "err", err)
		case <-fire:
			fire = nil
			w.resync()
			select {
			case w.events <- struct{}{}:
			default:
			}
		}
	}
}
