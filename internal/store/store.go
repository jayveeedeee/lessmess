// Package store maintains an in-memory model of a changes/ tree,
// watches it for external edits, and performs safe write operations.
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

// Change is one entry of the changes/ tree.
type Change struct {
	ID       string
	Dir      string // absolute path
	Archived bool
	Ledger   *model.ChangeLedger
	Err      error // non-nil if the ledger failed to parse
	Roots     []*TaskNode          // top-level tasks in change-ledger row order
	Nodes     map[string]*TaskNode // every task node by ID
	StrayDirs []string             // dirs under tasks/ without a matching task file
}

// Event notifies listeners that the store was reloaded or written.
type Event struct {
	Kind string // "fs" for external change, "write" for a store write
	Path string
}

var (
	// ErrNotFound is returned for unknown change or task IDs.
	ErrNotFound = errors.New("not found")
	// ErrInvalid is returned when a write targets a file that fails to parse.
	ErrInvalid = errors.New("file failed validation")
	// ErrNoChanges is returned by Open when the repository has no changes/
	// directory yet (the setup-mode trigger); a malformed tree is a
	// different, non-sentinel error.
	ErrNoChanges = errors.New("changes/ directory not found")
	// ErrNoContainer is returned when a subtask operation targets a task
	// that is not decomposed (no container directory).
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

	mu       sync.RWMutex
	root     *model.RootLedger
	rootErr  error
	changes  map[string]*Change
	archived map[string]*Change

	listeners map[chan Event]struct{}
	closed    chan struct{}
	closeOnce sync.Once

	// changeRoot resolves change directories that do not exist in the main
	// changes/ tree — the worktree-backed changes of the worktree pipeline.
	// Nil (the default) means every change lives in the main tree.
	changeRoot ChangeRootFunc
}

// ChangeRootFunc resolves the directory of one change. It is consulted
// during scan for root-ledger rows whose directory is absent from the main
// tree; ok=false or an empty dir leaves the row unmatched (a rule-6
// violation, as before).
type ChangeRootFunc func(id string) (dir string, ok bool)

// SetChangeRoot wires the resolver. Call once after Open; scan consults it
// on every reload. Pass nil to restore main-tree-only behavior.
func (s *Store) SetChangeRoot(fn ChangeRootFunc) {
	s.mu.Lock()
	s.changeRoot = fn
	s.mu.Unlock()
	s.Reload()
}

// Open scans the changes/ tree under dir.
func Open(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{
		Dir:        abs,
		ChangesDir: filepath.Join(abs, "changes"),
		changes:    map[string]*Change{},
		archived:   map[string]*Change{},
		listeners:  map[chan Event]struct{}{},
		closed:     make(chan struct{}),
	}
	if st, err := os.Stat(s.ChangesDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w under %s", ErrNoChanges, abs)
	}
	s.Reload()
	if s.rootErr != nil {
		// A missing root ledger is a partial (uninitialized) tree: setup
		// mode applies — init creates the ledger merge-safely. A ledger
		// that exists but does not parse stays a plain fatal error.
		if errors.Is(s.rootErr, os.ErrNotExist) {
			return nil, fmt.Errorf("changes/ tree incomplete (no root ledger at %s): %w",
				filepath.Join(s.ChangesDir, "ledger.md"), ErrNoChanges)
		}
		return nil, fmt.Errorf("root ledger: %w", s.rootErr)
	}
	return s, nil
}

// Reload rescans the whole tree (cheap at expected scale) and swaps the cache.
func (s *Store) Reload() {
	root, rootErr, changes, archived := s.scan()
	s.mu.Lock()
	s.root, s.rootErr, s.changes, s.archived = root, rootErr, changes, archived
	s.mu.Unlock()
}

func (s *Store) scan() (*model.RootLedger, error, map[string]*Change, map[string]*Change) {
	changes := map[string]*Change{}
	archived := map[string]*Change{}

	rootData, err := os.ReadFile(filepath.Join(s.ChangesDir, "ledger.md"))
	var root *model.RootLedger
	var rootErr error
	if err != nil {
		rootErr = err
	} else {
		root, rootErr = model.ParseRootLedger("changes/ledger.md", rootData)
	}

	load := func(dir string, archivedFlag bool) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			c := s.loadChange(e.Name(), filepath.Join(dir, e.Name()), archivedFlag)
			if archivedFlag {
				archived[c.ID] = c
			} else {
				changes[c.ID] = c
			}
		}
	}
	load(s.ChangesDir, false)
	load(filepath.Join(s.ChangesDir, "archive"), true)

	// Worktree-backed changes: active root rows whose directory exists
	// only inside the change's worktree resolve through the injected hook.
	// Rows already matched above (main tree or archive) are skipped, as
	// are cells that are not bare change IDs (archived rows keep their
	// archive-prefixed link in the Href column, not the Change cell, but
	// the ID-format check is cheap insurance).
	s.mu.RLock()
	resolve := s.changeRoot
	s.mu.RUnlock()
	if resolve != nil && root != nil {
		for _, r := range root.Rows {
			if _, ok := changes[r.Change]; ok {
				continue
			}
			if _, ok := archived[r.Change]; ok {
				continue
			}
			if !changeIDRe.MatchString(r.Change) {
				continue
			}
			dir, ok := resolve(r.Change)
			if !ok || dir == "" {
				continue
			}
			// A stale entry (worktree deleted by hand) self-heals to
			// unresolved rather than materializing a phantom change.
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				slog.Warn("resolved change root missing; skipping", "change", r.Change, "dir", dir)
				continue
			}
			changes[r.Change] = s.loadChange(r.Change, dir, false)
		}
	}
	return root, rootErr, changes, archived
}

// loadChange builds one Change (ledger, task tree) from its directory.
func (s *Store) loadChange(id, dir string, archivedFlag bool) *Change {
	c := &Change{
		ID:       id,
		Dir:      dir,
		Archived: archivedFlag,
	}
	if data, err := os.ReadFile(filepath.Join(c.Dir, "ledger.md")); err != nil {
		c.Err = err
	} else if l, err := model.ParseChangeLedger(c.ID+"/ledger.md", data); err != nil {
		c.Err = err
	} else {
		c.Ledger = l
	}
	var rows []model.TaskRow
	if c.Ledger != nil {
		rows = c.Ledger.Rows
	}
	c.Roots = s.scanTasks(c, filepath.Join(c.Dir, "tasks"), "tasks/", nil, rows)
	c.indexNodes()
	return c
}

// Root returns the parsed root ledger.
func (s *Store) Root() (*model.RootLedger, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root, s.rootErr
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
// Every write re-reads the target file from disk, applies the mutation to
// the fresh content, and writes atomically. External edits are therefore
// never clobbered: mutations apply on top of the latest on-disk state. A
// write is refused only when the file no longer parses (ErrInvalid) or the
// target row no longer exists (ErrNotFound).

func today() string { return time.Now().Format("2006-01-02") }

func (s *Store) freshLedger(c *Change) (*model.ChangeLedger, error) {
	data, err := os.ReadFile(filepath.Join(c.Dir, "ledger.md"))
	if err != nil {
		return nil, err
	}
	l, err := model.ParseChangeLedger(c.ID+"/ledger.md", data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return l, nil
}

func (s *Store) writeLedger(c *Change, l *model.ChangeLedger) error {
	if err := model.WriteFileAtomic(filepath.Join(c.Dir, "ledger.md"), l.Content(), 0o644); err != nil {
		return err
	}
	s.Reload()
	s.notify(Event{Kind: "write", Path: c.ID + "/ledger.md"})
	return nil
}

// governingLedgerPath returns the absolute path of the ledger that governs
// the given task node (the change ledger for top-level tasks, the parent
// container's ledger below that).
func governingLedgerPath(c *Change, n *TaskNode) string {
	if n.Parent == nil {
		return filepath.Join(c.Dir, "ledger.md")
	}
	return filepath.Join(c.Dir, filepath.FromSlash(n.Parent.ContainerRel()), "ledger.md")
}

// writeGoverningLedger atomically writes a governing-ledger file and
// reloads + notifies with the change-relative path.
func (s *Store) writeGoverningLedger(c *Change, n *TaskNode, content []byte) error {
	p := governingLedgerPath(c, n)
	if err := model.WriteFileAtomic(p, content, 0o644); err != nil {
		return err
	}
	rel := "ledger.md"
	if n.Parent != nil {
		rel = n.Parent.ContainerRel() + "/ledger.md"
	}
	s.Reload()
	s.notify(Event{Kind: "write", Path: rel})
	return nil
}

// MoveTask sets a task's status and position within its status group in
// the ledger that governs it (change ledger or parent container ledger).
func (s *Store) MoveTask(changeID, taskID string, toStatus model.TaskStatus, toIndex int) error {
	c, err := s.Change(changeID)
	if err != nil {
		return err
	}
	n := c.Node(taskID)
	if n == nil {
		return ErrNotFound
	}
	data, err := os.ReadFile(governingLedgerPath(c, n))
	if err != nil {
		return err
	}
	var content []byte
	if n.Parent == nil {
		l, err := model.ParseChangeLedger(c.ID+"/ledger.md", data)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		if err := l.MoveTask(taskID, toStatus, toIndex, today()); err != nil {
			if errors.Is(err, model.ErrTaskNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		content = l.Content()
	} else {
		l, err := model.ParseTaskLedger(c.ID+"/"+n.Parent.ContainerRel()+"/ledger.md", data)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		if err := l.MoveTask(taskID, toStatus, toIndex, today()); err != nil {
			if errors.Is(err, model.ErrTaskNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		content = l.Content()
	}
	if err := s.writeGoverningLedger(c, n, content); err != nil {
		return err
	}
	// The change's overall status is derived from the task tree.
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

// DecomposeTask turns a task into a sub plan: it creates the task's
// container directory with a minimal ledger.md and an empty tasks/
// directory. The task file itself is untouched and its row stays in the
// governing ledger. Returns the change-relative container path.
func (s *Store) DecomposeTask(changeID, taskID string) (string, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return "", err
	}
	n := c.Node(taskID)
	if n == nil {
		return "", ErrNotFound
	}
	containerRel := n.ContainerRel()
	dir := filepath.Join(c.Dir, filepath.FromSlash(containerRel))
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return "", ErrContainerExists
	}
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		return "", err
	}
	if err := model.WriteFileAtomic(filepath.Join(dir, "ledger.md"), model.RenderTaskLedger(taskID, changeID, today()), 0o644); err != nil {
		return "", err
	}
	s.Reload()
	s.notify(Event{Kind: "write", Path: containerRel + "/ledger.md"})
	return containerRel, nil
}

// CreateTask allocates the next task number within its governing level
// (the change root when parentTaskID is empty, the parent's container
// otherwise), writes the task file from the template, and appends the
// row to the governing ledger.
func (s *Store) CreateTask(changeID, parentTaskID, title string) (model.TaskRow, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return model.TaskRow{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" || strings.Contains(title, "|") {
		return model.TaskRow{}, fmt.Errorf("%w: task title must be non-empty and contain no |", ErrInvalid)
	}

	var tasksDir, relPrefix, ledgerPath string
	var parent *TaskNode
	if parentTaskID == "" {
		tasksDir = filepath.Join(c.Dir, "tasks")
		relPrefix = "tasks/"
		ledgerPath = filepath.Join(c.Dir, "ledger.md")
	} else {
		parent = c.Node(parentTaskID)
		if parent == nil {
			return model.TaskRow{}, ErrNotFound
		}
		containerRel := parent.ContainerRel()
		if _, err := os.Stat(filepath.Join(c.Dir, filepath.FromSlash(containerRel), "ledger.md")); err != nil {
			return model.TaskRow{}, ErrNoContainer
		}
		tasksDir = filepath.Join(c.Dir, filepath.FromSlash(containerRel), "tasks")
		relPrefix = containerRel + "/tasks/"
		ledgerPath = filepath.Join(c.Dir, filepath.FromSlash(containerRel), "ledger.md")
	}

	// Next sequence = highest existing filename sequence + 1 (no reuse).
	maxSeq := -1
	entries, _ := os.ReadDir(tasksDir)
	for _, e := range entries {
		var n int
		if _, err := fmt.Sscanf(e.Name(), "%02d-", &n); err == nil && n > maxSeq {
			maxSeq = n
		}
	}
	seq := maxSeq + 1

	var id string
	if parent == nil {
		prefix := ""
		if root, rerr := s.Root(); rerr == nil && root != nil {
			for _, r := range root.Rows {
				if r.Change == changeID && r.Prefix != model.Empty {
					prefix = r.Prefix
				}
			}
		}
		if prefix != "" {
			id = fmt.Sprintf("%s-%02d", prefix, seq)
		} else {
			id = fmt.Sprintf("%02d", seq)
		}
	} else {
		id = model.ChildTaskID(parentTaskID, fmt.Sprintf("%02d", seq))
	}
	filename := fmt.Sprintf("%02d-%s.md", seq, slug(title))
	href := relPrefix + filename // change-relative (node view)

	if err := model.WriteFileAtomic(filepath.Join(tasksDir, filename), model.RenderTaskFile(id, title), 0o644); err != nil {
		return model.TaskRow{}, err
	}

	row := model.TaskRow{
		ID: id, Href: "tasks/" + filename, Title: title,
		Status: model.StatusNotStarted, Updated: today(), Notes: model.Empty,
	}
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		return model.TaskRow{}, err
	}
	var content []byte
	if parent == nil {
		l, err := model.ParseChangeLedger(c.ID+"/ledger.md", data)
		if err != nil {
			return model.TaskRow{}, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		l.AppendTask(row)
		content = l.Content()
	} else {
		l, err := model.ParseTaskLedger(c.ID+"/"+parent.ContainerRel()+"/ledger.md", data)
		if err != nil {
			return model.TaskRow{}, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		l.AppendTask(row)
		content = l.Content()
	}
	if err := model.WriteFileAtomic(ledgerPath, content, 0o644); err != nil {
		return model.TaskRow{}, err
	}
	row.Href = href // callers get the change-relative view
	s.Reload()
	s.notify(Event{Kind: "write", Path: "ledger.md"})
	// A task added to a closed change reopens it implicitly; on open
	// changes the derived status is usually unchanged and this no-ops.
	s.syncOverall(changeID)
	return row, nil
}

// CreateChange scaffolds a change directory in the main changes/ tree and
// appends the root-ledger row. It is CreateChangeAt with the main-tree
// changes directory and a freshly minted ID.
func (s *Store) CreateChange(title, prefix, branch, date string) (string, error) {
	if err := validateChangeArgs(title, date); err != nil {
		return "", err
	}
	id, err := s.MintChangeID(date)
	if err != nil {
		return "", err
	}
	return id, s.CreateChangeAt(id, s.ChangesDir, title, prefix, branch, date)
}

// MintChangeID mints a random five-character lowercase alphanumeric suffix
// ID unique for the date among existing directories (including archived
// ones) and root-ledger rows — worktree-backed changes have rows but no
// main-tree directory, so rows must count too.
func (s *Store) MintChangeID(date string) (string, error) {
	existing := map[string]bool{}
	for _, base := range []string{s.ChangesDir, filepath.Join(s.ChangesDir, "archive")} {
		entries, _ := os.ReadDir(base)
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), date+"-") {
				existing[e.Name()] = true
			}
		}
	}
	if root, err := s.Root(); err == nil && root != nil {
		for _, r := range root.Rows {
			if strings.HasPrefix(r.Change, date+"-") {
				existing[r.Change] = true
			}
		}
	}
	var id string
	for retry := 0; ; retry++ {
		if retry >= 10 {
			return "", fmt.Errorf("%w: could not mint a unique change id for %s", ErrInvalid, date)
		}
		candidate := date + "-" + randSuffix()
		if !existing[candidate] {
			id = candidate
			break
		}
		slog.Debug("change id collision; regenerating", "date", date)
	}
	return id, nil
}

// validateChangeArgs enforces the shared CreateChange argument rules.
func validateChangeArgs(title, date string) error {
	title = strings.TrimSpace(title)
	if title == "" || strings.Contains(title, "|") {
		return fmt.Errorf("%w: change title must be non-empty and contain no |", ErrInvalid)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(date) {
		return fmt.Errorf("%w: bad date %q", ErrInvalid, date)
	}
	return nil
}

// CreateChangeAt scaffolds a change whose docs live under changesRoot (the
// main changes/ directory, or a worktree's changes/ directory for
// worktree-backed changes) while the root-ledger row is always appended to
// the main tree's root ledger. The caller owns ID minting (MintChangeID)
// and any git setup, so a git failure can never leave a half-scaffolded
// change: with the ID minted first, the branch and worktree exist before
// the first workflow file is written.
func (s *Store) CreateChangeAt(id, changesRoot, title, prefix, branch, date string) error {
	if err := validateChangeArgs(title, date); err != nil {
		return err
	}
	if !changeIDRe.MatchString(id) {
		return fmt.Errorf("%w: malformed change id %q", ErrInvalid, id)
	}
	title = strings.TrimSpace(title)
	if prefix == "" {
		prefix = model.Empty
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.Contains(branch, "|") {
		branch = model.Empty
	}

	dir := filepath.Join(changesRoot, id)
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		return err
	}
	if err := model.WriteFileAtomic(filepath.Join(dir, "plan.md"), model.RenderChangePlan(id, title, date), 0o644); err != nil {
		return err
	}
	if err := model.WriteFileAtomic(filepath.Join(dir, "ledger.md"), model.RenderChangeLedger(id, date), 0o644); err != nil {
		return err
	}

	rootData, err := os.ReadFile(filepath.Join(s.ChangesDir, "ledger.md"))
	if err != nil {
		return err
	}
	root, err := model.ParseRootLedger("changes/ledger.md", rootData)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	root.AppendRow(model.RootRow{
		Change: id, Href: id + "/plan.md", Title: title, Prefix: prefix,
		Branch: branch, Status: model.OverallPlanned, Created: date, Updated: date,
	})
	if err := model.WriteFileAtomic(filepath.Join(s.ChangesDir, "ledger.md"), root.Content(), 0o644); err != nil {
		return err
	}
	s.mu.Lock()
	s.root = root
	s.mu.Unlock()
	s.Reload()
	s.notify(Event{Kind: "write", Path: "ledger.md"})
	return nil
}

// SetChangeStatus updates a change's overall status in both the per-change
// ledger and the root-ledger row (close → Done, reopen → In progress).
func (s *Store) SetChangeStatus(changeID string, status model.OverallStatus) error {
	if !status.Valid() {
		return fmt.Errorf("%w: invalid overall status %q", ErrInvalid, string(status))
	}
	c, err := s.Change(changeID)
	if err != nil {
		return err
	}
	l, err := s.freshLedger(c)
	if err != nil {
		return err
	}
	l.SetOverall(status, today())
	if err := s.writeLedger(c, l); err != nil {
		return err
	}

	rootPath := filepath.Join(s.ChangesDir, "ledger.md")
	rootData, err := os.ReadFile(rootPath)
	if err != nil {
		return err
	}
	root, err := model.ParseRootLedger("changes/ledger.md", rootData)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if err := root.Update(changeID, status, today()); err != nil {
		if errors.Is(err, model.ErrChangeNotFound) {
			return ErrNotFound
		}
		return err
	}
	if err := model.WriteFileAtomic(rootPath, root.Content(), 0o644); err != nil {
		return err
	}
	s.Reload()
	s.notify(Event{Kind: "write", Path: "ledger.md"})
	return nil
}

// TaskFile returns the parsed task file for a change, reading from disk.
func (s *Store) TaskFile(changeID, href string) (*model.TaskFile, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return nil, err
	}
	clean := filepath.Clean(filepath.FromSlash(href))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return nil, ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(c.Dir, clean))
	if err != nil {
		return nil, ErrNotFound
	}
	tf, err := model.ParseTaskFile(changeID+"/"+href, data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return tf, nil
}

// PlanFile returns the raw markdown of a change's plan.md.
func (s *Store) PlanFile(changeID string) (string, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(c.Dir, "plan.md"))
	if err != nil {
		return "", ErrNotFound
	}
	return string(data), nil
}

// LedgerFile returns the raw markdown of a change's ledger.md.
func (s *Store) LedgerFile(changeID string) (string, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(c.Dir, "ledger.md"))
	if err != nil {
		return "", ErrNotFound
	}
	return string(data), nil
}

// ContainerLedgerFile returns the raw markdown of a task container's
// ledger.md. href is change-relative and must be a tasks/…/ledger.md
// path; anything else is ErrNotFound.
func (s *Store) ContainerLedgerFile(changeID, href string) (string, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(href))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", ErrNotFound
	}
	slashed := filepath.ToSlash(clean)
	parts := strings.Split(slashed, "/")
	if len(parts) < 3 || parts[0] != "tasks" || parts[len(parts)-1] != "ledger.md" {
		return "", ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(c.Dir, clean))
	if err != nil {
		return "", ErrNotFound
	}
	return string(data), nil
}
