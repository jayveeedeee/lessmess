package store

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lessmess/internal/model"
)

// DecomposeTask prepares a task's container prose directory (its href
// minus .md, plus a tasks/ subdirectory). In the JSON model the sub plan
// itself is state (children with dotted IDs); no per-container ledger
// exists. Returns the change-relative container path.
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
	s.Reload()
	s.notify(Event{Kind: "write", Path: containerRel + "/tasks"})
	return containerRel, nil
}

// CreateTask allocates the next task number within its governing level
// (the change root when parentTaskID is empty, the parent's container
// otherwise), writes the prose file from the template, and appends the
// task to the change state.
func (s *Store) CreateTask(changeID, parentTaskID, title string) (model.TaskState, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return model.TaskState{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return model.TaskState{}, fmt.Errorf("%w: task title must be non-empty", ErrInvalid)
	}

	var tasksDir, relPrefix string
	if parentTaskID == "" {
		tasksDir = filepath.Join(c.Dir, "tasks")
		relPrefix = "tasks/"
	} else {
		parent := c.Node(parentTaskID)
		if parent == nil {
			return model.TaskState{}, ErrNotFound
		}
		containerRel := parent.ContainerRel()
		tasksDir = filepath.Join(c.Dir, filepath.FromSlash(containerRel), "tasks")
		if _, err := os.Stat(tasksDir); err != nil {
			return model.TaskState{}, ErrNoContainer
		}
		relPrefix = containerRel + "/tasks/"
	}

	st, err := s.freshState(c)
	if err != nil {
		return model.TaskState{}, err
	}

	// Next sequence = highest existing sequence at this level + 1 (no
	// reuse), from the authoritative state.
	maxSeq := -1
	for i := range st.Tasks {
		if st.Tasks[i].Parent == parentTaskID && st.Tasks[i].Seq > maxSeq {
			maxSeq = st.Tasks[i].Seq
		}
	}
	seq := maxSeq + 1

	var id string
	if parentTaskID == "" {
		prefix := ""
		if e := s.Entry(changeID); e != nil {
			prefix = e.Prefix
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
	href := relPrefix + filename

	if err := model.WriteFileAtomic(filepath.Join(tasksDir, filename), model.RenderTaskFile(id, title), 0o644); err != nil {
		return model.TaskState{}, err
	}

	st.Tasks = append(st.Tasks, model.TaskState{
		ID: id, Seq: seq, Parent: parentTaskID, Title: title,
		File: href, Status: model.StatusNotStarted, Updated: today(),
	})
	st.Updated = today()
	if err := s.writeState(c, st); err != nil {
		return model.TaskState{}, err
	}
	// A task added to a closed change reopens it implicitly; on open
	// changes the derived status is usually unchanged and this no-ops.
	s.syncOverall(changeID)
	return model.TaskState{
		ID: id, Seq: seq, Parent: parentTaskID, Title: title,
		File: href, Status: model.StatusNotStarted, Updated: today(),
	}, nil
}

// CreateChange scaffolds a change in the main changes/ tree: prose
// directory plus JSON state plus index entry. It is CreateChangeAt with
// the main-tree changes directory and a freshly minted ID.
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
// ID unique for the date among existing prose directories (including
// archived ones) and index entries — worktree-backed changes have entries
// but no main-tree directory, so entries must count too.
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
	if idx, err := s.Index(); err == nil && idx != nil {
		for _, e := range idx.Changes {
			if strings.HasPrefix(e.ID, date+"-") {
				existing[e.ID] = true
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
	}
	return id, nil
}

// validateChangeArgs enforces the shared CreateChange argument rules.
func validateChangeArgs(title, date string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("%w: change title must be non-empty", ErrInvalid)
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("%w: bad date %q", ErrInvalid, date)
	}
	return nil
}

// CreateChangeAt scaffolds a change whose prose lives under changesRoot
// (the main changes/ directory, or a worktree's changes/ directory for
// worktree-backed changes) while the state file and index entry are
// always written to the main tree's workflow store. The caller owns ID
// minting (MintChangeID) and any git setup.
func (s *Store) CreateChangeAt(id, changesRoot, title, prefix, branch, date string) error {
	if err := validateChangeArgs(title, date); err != nil {
		return err
	}
	if !changeIDRe.MatchString(id) {
		return fmt.Errorf("%w: malformed change id %q", ErrInvalid, id)
	}
	title = strings.TrimSpace(title)
	prefix = strings.TrimSpace(prefix)
	branch = strings.TrimSpace(branch)

	dir := filepath.Join(changesRoot, id)
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		return err
	}
	if err := model.WriteFileAtomic(filepath.Join(dir, "plan.md"), model.RenderChangePlan(id, title, date), 0o644); err != nil {
		return err
	}
	st := &model.ChangeState{
		Version: model.StateVersion,
		ID:      id,
		Title:   title,
		Prefix:  prefix,
		Branch:  branch,
		Status:  model.ChangeStatus{Value: model.OverallPlanned, Derived: true},
		Created: date,
		Updated: date,
	}
	if err := os.MkdirAll(filepath.Join(s.WorkflowDir, "changes"), 0o755); err != nil {
		return err
	}
	if err := st.Save(s.statePath(id)); err != nil {
		return err
	}

	idx, err := s.freshIndex()
	if err != nil {
		return err
	}
	idx.Changes = append(idx.Changes, model.IndexEntry{
		ID: id, Title: title, Prefix: prefix, Branch: branch, Created: date,
	})
	if err := s.writeIndex(idx); err != nil {
		return err
	}
	return nil
}

// freshIndex re-reads and parses the workflow index.
func (s *Store) freshIndex() (*model.WorkflowIndex, error) {
	idx, err := model.LoadWorkflowIndex(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return idx, nil
}

// SetChangeStatus sets a change's overall status in its state file
// (close → Done, reopen → In progress). Tool-derivable statuses are
// stored derived; Done and Cancelled are user-set and exempt from
// recomputation.
func (s *Store) SetChangeStatus(changeID string, status model.OverallStatus) error {
	if !status.Valid() {
		return fmt.Errorf("%w: invalid overall status %q", ErrInvalid, string(status))
	}
	c, err := s.Change(changeID)
	if err != nil {
		return err
	}
	st, err := s.freshState(c)
	if err != nil {
		return err
	}
	st.Status = model.ChangeStatus{Value: status, Derived: derivableOverall(status)}
	st.Updated = today()
	return s.writeState(c, st)
}

// derivableOverall reports whether an overall status is one the tool
// derives from the task tree (as opposed to user-set Done/Cancelled).
func derivableOverall(s model.OverallStatus) bool {
	switch s {
	case model.OverallPlanned, model.OverallInProgress, model.OverallBlocked:
		return true
	}
	return false
}

// syncOverall recomputes one change's derived overall status after a
// task-level write (user-set statuses are exempt).
func (s *Store) syncOverall(changeID string) {
	s.mu.RLock()
	c, ok := s.changes[changeID]
	s.mu.RUnlock()
	if !ok || c.State == nil || !c.State.Status.Derived {
		return
	}
	if c.State.Status.Value == DeriveOverall(c) {
		return
	}
	st, err := s.freshState(c)
	if err != nil {
		return
	}
	if !st.Status.Derived {
		return // externally set to a user status; leave it
	}
	st.Status.Value = DeriveOverall(c)
	st.Updated = today()
	if err := s.writeState(c, st); err != nil {
		slog.Warn("sync overall", "change", changeID, "err", err)
	}
}

// SyncOverallStatuses converges every change's derived overall status to
// its task tree (user-set statuses are exempt).
func (s *Store) SyncOverallStatuses() {
	for _, c := range s.Changes() {
		s.syncOverall(c.ID)
	}
}

// SetTaskStatus writes one task's status and appends optional evidence to
// its notes, then converges the derived overall status. Vocabulary and
// existence are enforced (ErrInvalid/ErrNotFound); the user gate for Done
// is the handler's caller-identity concern.
func (s *Store) SetTaskStatus(changeID, taskID string, status model.TaskStatus, evidence string) (model.TaskState, error) {
	if !status.Valid() {
		return model.TaskState{}, fmt.Errorf("%w: invalid status %q", ErrInvalid, string(status))
	}
	c, err := s.Change(changeID)
	if err != nil {
		return model.TaskState{}, err
	}
	st, err := s.freshState(c)
	if err != nil {
		return model.TaskState{}, err
	}
	ts := st.Task(taskID)
	if ts == nil {
		return model.TaskState{}, ErrNotFound
	}
	// Test demands documented verification evidence: the request carries
	// it or the task's notes already do.
	if status == model.StatusTest && strings.TrimSpace(evidence) == "" && strings.TrimSpace(ts.Notes) == "" {
		return model.TaskState{}, fmt.Errorf("%w: Test transition requires verification evidence: pass evidence or record notes first", ErrInvalid)
	}
	ts.Status = status
	ts.Updated = today()
	if ev := strings.TrimSpace(evidence); ev != "" {
		if ts.Notes != "" {
			ts.Notes += " "
		}
		ts.Notes += ev
	}
	st.Updated = today()
	if err := s.writeState(c, st); err != nil {
		return model.TaskState{}, err
	}
	s.syncOverall(changeID)
	if fresh, _ := s.Change(changeID); fresh != nil && fresh.State != nil {
		if t := fresh.State.Task(taskID); t != nil {
			return *t, nil
		}
	}
	return *ts, nil
}

// TaskUpdate patches a task's metadata. Nil fields are left untouched;
// dependency existence and cycles are enforced by the state validation on
// write.
type TaskUpdate struct {
	Title     *string
	Notes     *string
	DependsOn *[]string
}

// UpdateTask applies a metadata patch to one task.
func (s *Store) UpdateTask(changeID, taskID string, upd TaskUpdate) (model.TaskState, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return model.TaskState{}, err
	}
	st, err := s.freshState(c)
	if err != nil {
		return model.TaskState{}, err
	}
	ts := st.Task(taskID)
	if ts == nil {
		return model.TaskState{}, ErrNotFound
	}
	if upd.Title != nil {
		title := strings.TrimSpace(*upd.Title)
		if title == "" {
			return model.TaskState{}, fmt.Errorf("%w: title must be non-empty", ErrInvalid)
		}
		ts.Title = title
	}
	if upd.Notes != nil {
		ts.Notes = strings.TrimSpace(*upd.Notes)
	}
	if upd.DependsOn != nil {
		deps := make([]string, 0, len(*upd.DependsOn))
		for _, d := range *upd.DependsOn {
			if d = strings.TrimSpace(d); d != "" {
				deps = append(deps, d)
			}
		}
		ts.DependsOn = deps
	}
	ts.Updated = today()
	st.Updated = today()
	if err := s.writeState(c, st); err != nil {
		return model.TaskState{}, err
	}
	return *ts, nil
}

// ReorderTasks sets one level's priority order (parent "" is top-level).
// ordered must list every task of that level exactly once.
func (s *Store) ReorderTasks(changeID, parent string, ordered []string) error {
	c, err := s.Change(changeID)
	if err != nil {
		return err
	}
	st, err := s.freshState(c)
	if err != nil {
		return err
	}
	level := st.Children(parent)
	if len(ordered) != len(level) {
		return fmt.Errorf("%w: reorder lists %d tasks, level has %d", ErrInvalid, len(ordered), len(level))
	}
	rank := map[string]int{}
	for i, id := range ordered {
		if _, dup := rank[id]; dup {
			return fmt.Errorf("%w: reorder lists %s twice", ErrInvalid, id)
		}
		rank[id] = i
	}
	// Stable reorder inside the flat array: collect the level's positions
	// and their new values in the requested order.
	positions := make([]int, 0, len(level))
	for i := range st.Tasks {
		if st.Tasks[i].Parent == parent {
			positions = append(positions, i)
		}
	}
	newVals := make([]model.TaskState, 0, len(ordered))
	for _, id := range ordered {
		ts := st.Task(id)
		if ts == nil || ts.Parent != parent {
			return fmt.Errorf("%w: %s is not a task at this level", ErrInvalid, id)
		}
		newVals = append(newVals, *ts)
	}
	for i, arr := range positions {
		st.Tasks[arr] = newVals[i]
	}
	st.Updated = today()
	return s.writeState(c, st)
}

// AppendDecision appends one decision-log entry to a change.
func (s *Store) AppendDecision(changeID, date, decision string) error {
	c, err := s.Change(changeID)
	if err != nil {
		return err
	}
	decision = strings.TrimSpace(decision)
	if decision == "" {
		return fmt.Errorf("%w: decision must be non-empty", ErrInvalid)
	}
	if date == "" {
		date = today()
	}
	st, err := s.freshState(c)
	if err != nil {
		return err
	}
	st.Decision = append(st.Decision, model.DecisionEntry{Date: date, Decision: decision})
	st.Updated = today()
	return s.writeState(c, st)
}

// TaskFile returns the parsed view of one task prose file: identity from
// the JSON state (looked up by its change-relative href), body read raw
// from disk — prose files carry no frontmatter.
func (s *Store) TaskFile(changeID, href string) (*model.TaskFile, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return nil, err
	}
	clean := filepath.Clean(filepath.FromSlash(href))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return nil, ErrNotFound
	}
	slashed := filepath.ToSlash(clean)
	var t *model.TaskState
	if c.State != nil {
		for i := range c.State.Tasks {
			if c.State.Tasks[i].File == slashed {
				t = &c.State.Tasks[i]
				break
			}
		}
	}
	if t == nil {
		return nil, ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(c.Dir, clean))
	if err != nil {
		return nil, ErrNotFound
	}
	return &model.TaskFile{ID: t.ID, Title: t.Title, Body: string(data)}, nil
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
