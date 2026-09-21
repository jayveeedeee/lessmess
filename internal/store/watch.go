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

// Watch starts watching the workflow state and the prose trees until ctx
// is cancelled or the store is closed. External edits trigger a debounced
// Reload and an Event broadcast.
func (s *Store) Watch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	addDirs := func() { s.watchDirs(w) }
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
				// Attribute-only events (atime updates from readers like
				// git status, or chmod) are not content changes; reacting
				// to them makes read-heavy scans retrigger the UI forever.
				if ev.Op&^fsnotify.Chmod == 0 {
					continue
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
				// Converge derived overall statuses so external (agent)
				// state edits drift back automatically.
				s.SyncOverallStatuses()
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

// watchDirs watches the whole workflow state subtree plus every change's
// prose tree, including worktree-backed directories outside the main
// tree.
func (s *Store) watchDirs(w *fsnotify.Watcher) {
	addTree := func(root string) {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				if err := w.Add(p); err != nil {
					slog.Warn("watch add", "path", p, "err", err)
				}
			}
			return nil
		})
	}
	// The JSON state store is small: watch its whole subtree so external
	// edits (or git operations) reload the board.
	addTree(s.WorkflowDir)
	// Prose trees: the main changes/ tree plus worktree-backed dirs.
	addTree(s.ChangesDir)
	for _, c := range s.Changes() {
		if !strings.HasPrefix(c.Dir, s.ChangesDir) {
			addTree(c.Dir)
		}
	}
}
