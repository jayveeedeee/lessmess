package store

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debounceDelay coalesces bursts of filesystem events into one reload.
const debounceDelay = 150 * time.Millisecond

// Watch starts watching the changes/ tree (recursively) until ctx is
// cancelled or the store is closed. External edits trigger a debounced
// Reload and an Event broadcast.
func (s *Store) Watch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	addDirs := func() {
		_ = filepath.WalkDir(s.ChangesDir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				if err := w.Add(p); err != nil {
					slog.Warn("watch add", "path", p, "err", err)
				}
			}
			return nil
		})
	}
	addDirs()

	go func() {
		defer w.Close()
		var timer *time.Timer
		var debounce <-chan time.Time
		reset := func() {
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(debounceDelay)
			debounce = timer.C
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.closed:
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				// Ignore our own atomic-write temp files.
				if strings.HasPrefix(filepath.Base(ev.Name), ".tt-") {
					continue
				}
				reset()
			case <-debounce:
				debounce = nil
				addDirs() // pick up directories created since last scan
				s.Reload()
				s.notify(Event{Kind: "fs"})
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				slog.Warn("watch", "err", err)
			}
		}
	}()
	return nil
}
