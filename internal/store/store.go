// Package store maintains an in-memory model of the workflow state (the
// JSON store under .lessmess/workflow/ plus the prose tree under
// changes/), watches both for external edits, and performs safe write
// operations.
package store

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"lessmess/internal/model"
)

// Change is one entry of the workflow: its JSON state plus its prose
// directory under changes/ (main tree, archive, or a worktree).
type Change struct {
	ID          string
	Dir         string // absolute path of the prose directory
	Archived    bool
	State       *model.ChangeState // nil with Err set when the state file fails to load
	Err         error
	Roots       []*TaskNode          // top-level tasks in priority order
	Nodes       map[string]*TaskNode // every task node by ID
	OrphanFiles []string             // prose files under tasks/ not referenced by state
	StrayDirs   []string             // dirs under tasks/ not referenced by state
	containers  map[string]bool      // change-relative container dirs (see scanProse)
}

// Overall returns the change's overall status (Planned when the state
// failed to load).
func (c *Change) Overall() model.OverallStatus {
	if c.State == nil {
		return model.OverallPlanned
	}
	return c.State.Status.Value
}

// Title returns the change title from its state.
func (c *Change) Title() string {
	if c.State == nil {
		return c.ID
	}
	return c.State.Title
}

// Event notifies listeners that the store was reloaded or written.
type Event struct {
	Kind string // "fs" for external change, "write" for a store write
	Path string
}

var (
	// ErrNotFound is returned for unknown change or task IDs.
	ErrNotFound = errors.New("not found")
	// ErrInvalid is returned when a write targets state that fails to parse.
	ErrInvalid = errors.New("file failed validation")
	// ErrNoChanges is returned by Open when the repository has no workflow
	// state yet (the setup-mode trigger); a malformed store is a
	// different, non-sentinel error.
	ErrNoChanges = errors.New("workflow state not found")
	// ErrLegacyMarkdown is returned by Open when the repository still has
	// its state in markdown ledgers; run MigrateWorkflow (lessmess
	// migrate, or restart serve which auto-migrates) first.
	ErrLegacyMarkdown = errors.New("workflow state is still markdown")
	// ErrNoContainer is returned when a subtask operation targets a task
	// that has no container prose directory.
	ErrNoContainer = errors.New("task is not decomposed")
	// ErrContainerExists is returned by DecomposeTask when the task
	// already has a container.
	ErrContainerExists = errors.New("task already has a sub plan")
)

// changeIDRe accepts both change-ID formats: the legacy numeric suffix
// (any digits, never reused for new changes) and the current five-character
// lowercase alphanumeric suffix.
var changeIDRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-(\d+|[a-z0-9]{5})$`)

// randSuffix mints one random five-character lowercase alphanumeric suffix
// with crypto/rand. Package-level so tests can force deterministic
// sequences; the modulo fold has a negligible bias that is irrelevant here
// (IDs are uniqueness tokens, not secrets).
var randSuffix = func() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		panic("store: crypto/rand unavailable: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// Store is the in-memory model plus watcher for one repository.
type Store struct {
	Dir        string // repository root
	ChangesDir string
	// WorkflowDir is .lessmess/workflow: the JSON state store.
	WorkflowDir string

	mu         sync.RWMutex
	index      *model.WorkflowIndex
	indexErr   error
	changes    map[string]*Change
	archived   map[string]*Change
	unresolved map[string]*Change // index entries with no prose directory
	listeners  map[chan Event]struct{}
	closed     chan struct{}
	closeOnce  sync.Once

	// changeRoot resolves prose directories that do not exist in the main
	// changes/ tree — the worktree-backed changes of the worktree
	// pipeline. Nil (the default) means every change lives in the main
	// tree. The JSON state is always main-tree.
	changeRoot ChangeRootFunc
}

// ChangeRootFunc resolves the prose directory of one change. It is
// consulted during scan for index entries whose directory is absent from
// the main tree; ok=false or an empty dir leaves the entry unresolved
// (a validation violation).
type ChangeRootFunc func(id string) (dir string, ok bool)

// SetChangeRoot wires the resolver. Call once after Open; scan consults it
// on every reload. Pass nil to restore main-tree-only behavior.
func (s *Store) SetChangeRoot(fn ChangeRootFunc) {
	s.mu.Lock()
	s.changeRoot = fn
	s.mu.Unlock()
	s.Reload()
}

// Open loads the workflow state under dir. A repository whose state is
// still markdown (changes/ledger.md without .lessmess/workflow/) fails
// with ErrLegacyMarkdown; a repository with neither is ErrNoChanges
// (setup mode).
func Open(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{
		Dir:         abs,
		ChangesDir:  filepath.Join(abs, "changes"),
		WorkflowDir: filepath.Join(abs, StateDirName, "workflow"),
		changes:     map[string]*Change{},
		archived:    map[string]*Change{},
		unresolved:  map[string]*Change{},
		listeners:   map[chan Event]struct{}{},
		closed:      make(chan struct{}),
	}
	if _, err := os.Stat(s.indexPath()); err != nil {
		if _, lerr := os.Stat(filepath.Join(s.ChangesDir, "ledger.md")); lerr == nil {
			return nil, fmt.Errorf("%w: %s exists without %s (run 'lessmess migrate')",
				ErrLegacyMarkdown, filepath.Join(s.ChangesDir, "ledger.md"), s.indexPath())
		}
		return nil, fmt.Errorf("%w under %s (no %s)", ErrNoChanges, abs, s.indexPath())
	}
	s.Reload()
	if s.indexErr != nil {
		return nil, fmt.Errorf("workflow index: %w", s.indexErr)
	}
	return s, nil
}

func (s *Store) indexPath() string { return filepath.Join(s.WorkflowDir, "index.json") }
func (s *Store) statePath(id string) string {
	return filepath.Join(s.WorkflowDir, "changes", id+".json")
}

// Reload rescans the whole state (cheap at expected scale) and swaps the cache.
func (s *Store) Reload() {
	index, indexErr, changes, archived := s.scan()
	s.mu.Lock()
	s.index, s.indexErr, s.changes, s.archived = index, indexErr, changes, archived
	s.mu.Unlock()
}

func (s *Store) scan() (*model.WorkflowIndex, error, map[string]*Change, map[string]*Change) {
	changes := map[string]*Change{}
	archived := map[string]*Change{}
	unresolved := map[string]*Change{}

	index, indexErr := model.LoadWorkflowIndex(s.indexPath())

	s.mu.RLock()
	resolve := s.changeRoot
	s.mu.RUnlock()
	if index != nil {
		for i := range index.Changes {
			e := &index.Changes[i]
			dir, ok := s.proseDir(e, resolve)
			c := s.loadChange(e.ID, dir, ok, e.Archived)
			if !ok {
				// Unresolved entries are not changes: Change() reports
				// ErrNotFound and Validate flags them (rule 6). The
				// resolver may heal them on a later reload.
				unresolved[c.ID] = c
				continue
			}
			if e.Archived {
				archived[c.ID] = c
			} else {
				changes[c.ID] = c
			}
		}
	}
	s.mu.Lock()
	s.unresolved = unresolved
	s.mu.Unlock()
	return index, indexErr, changes, archived
}

// proseDir resolves the prose directory of one index entry: the main
// tree, then changes/archive/, then the worktree resolver.
func (s *Store) proseDir(e *model.IndexEntry, resolve ChangeRootFunc) (string, bool) {
	candidates := []string{filepath.Join(s.ChangesDir, e.ID)}
	if e.Archived {
		candidates = []string{filepath.Join(s.ChangesDir, "archive", e.ID)}
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p, true
		}
	}
	if resolve != nil {
		if dir, ok := resolve(e.ID); ok && dir != "" {
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				return dir, true
			}
			slog.Warn("resolved change root missing; skipping", "change", e.ID, "dir", dir)
		}
	}
	return "", false
}

// loadChange builds one Change from its JSON state and prose directory.
// Unresolved prose directories yield a Change with Dir == "" (a
// validation violation, not a load failure).
func (s *Store) loadChange(id, dir string, dirOK, archivedFlag bool) *Change {
	c := &Change{ID: id, Archived: archivedFlag}
	if !dirOK {
		c.Dir = ""
		return c
	}
	c.Dir = dir
	st, err := model.LoadChangeState(s.statePath(id))
	if err != nil {
		c.Err = err
	} else {
		c.State = st
		c.scanProse()
		c.buildTree()
	}
	return c
}

// buildTree constructs the node tree from the flat task array. Array
// order is priority order within each level (Parent "" is top-level).
func (c *Change) buildTree() {
	c.Roots = nil
	c.Nodes = map[string]*TaskNode{}
	if c.State == nil {
		return
	}
	for i := range c.State.Tasks {
		t := &c.State.Tasks[i]
		c.Nodes[t.ID] = &TaskNode{ID: t.ID, Href: t.File, Task: t}
	}
	for i := range c.State.Tasks {
		t := &c.State.Tasks[i]
		n := c.Nodes[t.ID]
		if t.Parent == "" {
			c.Roots = append(c.Roots, n)
			continue
		}
		if p := c.Nodes[t.Parent]; p != nil {
			p.Children = append(p.Children, n)
			n.Parent = p
		}
	}
	// Mark decomposed tasks whose container prose directory exists (it
	// may be empty until the first subtask lands).
	for _, n := range c.Nodes {
		if c.containers[strings.TrimSuffix(n.Href, ".md")] {
			n.containerDir = true
		}
	}
}

// Index returns the parsed workflow index.
func (s *Store) Index() (*model.WorkflowIndex, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index, s.indexErr
}

// Entry returns the index entry for one change, or nil.
func (s *Store) Entry(id string) *model.IndexEntry {
	idx, err := s.Index()
	if err != nil || idx == nil {
		return nil
	}
	return idx.Find(id)
}

// Changes returns active changes sorted by ID.
func (s *Store) Changes() []*Change {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sortedChanges(s.changes)
}

// Archived returns archived changes sorted by ID.
func (s *Store) Archived() []*Change {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sortedChanges(s.archived)
}

// all returns active plus archived changes (unsorted map copy).
func (s *Store) all() map[string]*Change {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*Change, len(s.changes)+len(s.archived))
	for k, v := range s.changes {
		out[k] = v
	}
	for k, v := range s.archived {
		out[k] = v
	}
	return out
}

func sortedChanges(m map[string]*Change) []*Change {
	out := make([]*Change, 0, len(m))
	for _, c := range m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Change returns the change with the given ID (active set only).
func (s *Store) Change(id string) (*Change, error) {
	if !changeIDRe.MatchString(id) {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.changes[id]; ok {
		return c, nil
	}
	return nil, ErrNotFound
}

// Subscribe registers a listener for store events.
func (s *Store) Subscribe() chan Event {
	ch := make(chan Event, 8)
	s.mu.Lock()
	s.listeners[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes a listener.
func (s *Store) Unsubscribe(ch chan Event) {
	s.mu.Lock()
	delete(s.listeners, ch)
	s.mu.Unlock()
}

// Close releases resources (idempotent).
func (s *Store) Close() { s.closeOnce.Do(func() { close(s.closed) }) }

// Done returns a channel closed on Close.
func (s *Store) Done() <-chan struct{} { return s.closed }

func (s *Store) notify(ev Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.listeners {
		select {
		case ch <- ev:
		default:
		}
	}
}

// --- write operations ---
//
// Every write re-reads the target state file from disk, applies the
// mutation to the fresh content, validates it, and writes atomically.
// External edits are therefore never clobbered: mutations apply on top of
// the latest on-disk state.

func today() string { return time.Now().Format("2006-01-02") }

// freshState re-reads and parses one change's state file.
func (s *Store) freshState(c *Change) (*model.ChangeState, error) {
	st, err := model.LoadChangeState(s.statePath(c.ID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return st, nil
}

// writeState validates and atomically writes one change's state file,
// then reloads and notifies.
func (s *Store) writeState(c *Change, st *model.ChangeState) error {
	if issues := st.Validate(); len(issues) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(issues, "; "))
	}
	if err := st.Save(s.statePath(c.ID)); err != nil {
		return err
	}
	s.Reload()
	s.notify(Event{Kind: "write", Path: c.ID + "/state"})
	return nil
}

// writeIndex validates and atomically writes the workflow index, then
// reloads and notifies.
func (s *Store) writeIndex(idx *model.WorkflowIndex) error {
	if issues := idx.Validate(); len(issues) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(issues, "; "))
	}
	if err := idx.Save(s.indexPath()); err != nil {
		return err
	}
	s.Reload()
	s.notify(Event{Kind: "write", Path: "index"})
	return nil
}

// MoveTask sets a task's status and position within its status group at
// its level of the task tree.
func (s *Store) MoveTask(changeID, taskID string, toStatus model.TaskStatus, toIndex int) error {
	if !toStatus.Valid() {
		return fmt.Errorf("%w: invalid status %q", ErrInvalid, string(toStatus))
	}
	c, err := s.Change(changeID)
	if err != nil {
		return err
	}
	// The fresh state is authoritative: a cache miss does not mean the
	// task is gone (the cache may lag an external edit), and a corrupt
	// state file must surface as ErrInvalid, not ErrNotFound.
	st, err := s.freshState(c)
	if err != nil {
		return err
	}
	if st.Task(taskID) == nil {
		return ErrNotFound
	}
	parent := st.Task(taskID).Parent
	// The level's flat positions and values, in array (priority) order.
	var levelPos []int
	for i := range st.Tasks {
		if st.Tasks[i].Parent == parent {
			levelPos = append(levelPos, i)
		}
	}
	level := make([]model.TaskState, len(levelPos))
	for li, arr := range levelPos {
		level[li] = st.Tasks[arr]
	}
	pos := -1
	for li := range level {
		if level[li].ID == taskID {
			pos = li
			break
		}
	}
	if pos < 0 {
		return ErrNotFound
	}
	// Remove the moved task, set its new status, and reinsert at visual
	// index toIndex within its status group (the markdown model's
	// insertionPos semantics, preserved).
	moving := level[pos]
	moving.Status = toStatus
	moving.Updated = today()
	rest := make([]model.TaskState, 0, len(level)-1)
	rest = append(rest, level[:pos]...)
	rest = append(rest, level[pos+1:]...)
	count := 0
	at := len(rest)
	for i := range rest {
		if rest[i].Status != toStatus {
			continue
		}
		if count == toIndex {
			at = i
			break
		}
		count++
	}
	if count > 0 && at == len(rest) {
		// Index beyond the group: land after the group's last member.
		for i := len(rest) - 1; i >= 0; i-- {
			if rest[i].Status == toStatus {
				at = i + 1
				break
			}
		}
	}
	rest = append(rest, model.TaskState{})
	copy(rest[at+1:], rest[at:])
	rest[at] = moving
	// Write the level back into its flat positions.
	for li := range levelPos {
		st.Tasks[levelPos[li]] = rest[li]
	}
	st.Updated = today()
	if err := s.writeState(c, st); err != nil {
		return err
	}
	s.syncOverall(changeID)
	return nil
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slug(title string) string {
	s := slugRe.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" {
		s = "task"
	}
	return s
}
