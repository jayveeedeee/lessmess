package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// autosession is the once-only marker for auto-spawned task sessions
// (.lessmess/autosession.json): one entry per decomposed task node that
// already got its session. Unlinking a session later never respawns
// because the marker stays set. Spawn failures leave the marker unset so
// the next surface retry (board render or expand) can try again.
type autosession struct {
	path string
	mu   sync.Mutex
	done map[string]bool // key: changeID + "\x00" + taskID
}

func loadAutosession(path string) *autosession {
	a := &autosession{path: path, done: map[string]bool{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return a // absent state is normal
	}
	var keys []string
	if json.Unmarshal(b, &keys) == nil {
		for _, k := range keys {
			a.done[k] = true
		}
	}
	return a
}

func (a *autosession) save() error {
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	keys := make([]string, 0, len(a.done))
	for k := range a.done {
		keys = append(keys, k)
	}
	b, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return model.WriteFileAtomic(a.path, append(b, '\n'), 0o644)
}

func (a *autosession) has(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.done[key]
}

func (a *autosession) mark(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done[key] {
		return nil
	}
	a.done[key] = true
	return a.save()
}

// autoSpawnMu collapses concurrent sweeps: a sweep already in flight makes
// new ones skip (TryLock) rather than queue behind it.
var autoSpawnMu sync.Mutex

// autospawnSweep spawns task-scoped sessions for decomposed tasks that
// have neither a marker nor a bound session, across all active changes.
// Bounded so one sweep cannot stampede; failures log and leave the marker
// unset for the next sweep. No-ops without the service or with an
// unreadable mapping.
func (s *Server) autospawnSweep() {
	for _, c := range s.st.Changes() {
		s.autospawnChange(c, 3)
	}
}

// autospawnChange spawns for one change's unmarked container nodes.
func (s *Server) autospawnChange(c *store.Change, limit int) {
	if s.oc == nil || s.mapErr != nil {
		return
	}
	if !autoSpawnMu.TryLock() {
		return
	}
	defer autoSpawnMu.Unlock()
	bound := map[string]bool{}
	for _, e := range s.sessions.list(c.ID) {
		if e.Task != "" {
			bound[e.Task] = true
		}
	}
	spawned := 0
	c.WalkTasks(func(n *store.TaskNode) bool {
		if spawned >= limit {
			return false
		}
		if !n.HasContainer() || n.ID == "" {
			return true
		}
		key := c.ID + "\x00" + n.ID
		if s.autos.has(key) {
			return true
		}
		// A session bound by other means (delegation, manual bind) counts
		// as done: mark so a later unlink does not respawn.
		if bound[n.ID] {
			if err := s.autos.mark(key); err != nil {
				slog.Warn("autosession mark", "err", err)
			}
			return true
		}
		if err := s.autospawnOne(c, n); err != nil {
			slog.Warn("task session auto-spawn failed", "change", c.ID, "task", n.ID, "err", err)
			return true // marker stays unset; retried on a later sweep
		}
		spawned++
		return true
	})
}

// autospawnOne spawns, primes, and maps one task session, then marks it.
func (s *Server) autospawnOne(c *store.Change, n *store.TaskNode) error {
	title := taskSessionTitle(n)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sess, err := s.spawnSessionIn(ctx, s.changeSessionDir(c.ID), title)
	if err != nil {
		return err
	}
	prime, modules := s.taskPrime(c.ID, n, sess.ID)
	if err := s.oc.Prompt(ctx, sess.ID, s.promptWith(prime, "change")); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		return errors.New("prime task session: " + err.Error())
	}
	logPrime(sess.ID, "task", modules)
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339), Task: n.ID, Modules: modules}
	if err := s.sessions.add(c.ID, entry); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		return errors.New("persist mapping: " + err.Error())
	}
	key := c.ID + "\x00" + n.ID
	if err := s.autos.mark(key); err != nil {
		slog.Warn("autosession mark", "err", err)
	}
	slog.Info("task session auto-spawned", "change", c.ID, "task", n.ID, "session", sess.ID)
	return nil
}

// taskSessionTitle builds the "<task-id>: <title>" title; the dotted
// prefix makes delegation-style reconciliation recognize it too.
func taskSessionTitle(n *store.TaskNode) string {
	title := n.ID
	if n.Task != nil && n.Task.Title != "" {
		title = n.Task.Title
	}
	return n.ID + ": " + title
}
