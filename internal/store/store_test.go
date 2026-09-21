package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"lessmess/internal/model"
)

// writeFixture builds a minimal valid JSON workflow store in dir: one
// change with two tasks (prose files included).
func writeFixture(t *testing.T, dir string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	cdir := filepath.Join(dir, "changes", "2026-09-10-0")
	must(os.MkdirAll(filepath.Join(cdir, "tasks"), 0o755))
	must(os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan("2026-09-10-0", "Fixture change", "2026-09-10"), 0o644))
	must(os.WriteFile(filepath.Join(cdir, "tasks", "00-first.md"), model.RenderTaskFile("FIX-00", "First"), 0o644))
	must(os.WriteFile(filepath.Join(cdir, "tasks", "01-second.md"), model.RenderTaskFile("FIX-01", "Second"), 0o644))

	st := &model.ChangeState{
		Version: model.StateVersion,
		ID:      "2026-09-10-0",
		Title:   "Fixture change",
		Prefix:  "FIX",
		Status:  model.ChangeStatus{Value: model.OverallInProgress, Derived: true},
		Created: "2026-09-10",
		Updated: "2026-09-10",
		Tasks: []model.TaskState{
			{ID: "FIX-00", Seq: 0, Title: "First", File: "tasks/00-first.md", Status: model.StatusNotStarted, Updated: "2026-09-10"},
			{ID: "FIX-01", Seq: 1, Title: "Second", File: "tasks/01-second.md", Status: model.StatusInProgress, Updated: "2026-09-10"},
		},
	}
	wd := filepath.Join(dir, StateDirName, "workflow")
	must(os.MkdirAll(filepath.Join(wd, "changes"), 0o755))
	must(st.Save(filepath.Join(wd, "changes", "2026-09-10-0.json")))
	idx := &model.WorkflowIndex{
		Version: model.StateVersion,
		Changes: []model.IndexEntry{{ID: "2026-09-10-0", Title: "Fixture change", Prefix: "FIX", Created: "2026-09-10"}},
	}
	must(idx.Save(filepath.Join(wd, "index.json")))
}

func openFixture(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, dir)
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(s.Close)
	return s, dir
}

func TestLoadValid(t *testing.T) {
	s, _ := openFixture(t)
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations = %v", v)
	}
	changes := s.Changes()
	if len(changes) != 1 || changes[0].ID != "2026-09-10-0" {
		t.Fatalf("changes = %+v", changes)
	}
	c := changes[0]
	if len(c.State.Tasks) != 2 || c.State.Tasks[0].ID != "FIX-00" {
		t.Fatalf("tasks = %+v", c.State.Tasks)
	}
	if len(c.Roots) != 2 || c.Roots[0].Task.Title != "First" {
		t.Fatalf("roots = %+v", c.Roots)
	}
	if c.Overall() != model.OverallInProgress {
		t.Fatalf("overall = %q", c.Overall())
	}
}

func TestValidateBadDirName(t *testing.T) {
	s, dir := openFixture(t)
	if err := os.MkdirAll(filepath.Join(dir, "changes", "bogus-dir", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 1, "bogus-dir")
}

func TestValidateMissingPlan(t *testing.T) {
	s, dir := openFixture(t)
	if err := os.Remove(filepath.Join(dir, "changes", "2026-09-10-0", "plan.md")); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 2, "plan.md")
}

func TestValidateOrphanTaskFile(t *testing.T) {
	s, dir := openFixture(t)
	p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "02-orphan.md")
	if err := os.WriteFile(p, []byte("# orphan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 3, "not referenced")
}

func TestValidateMissingProseFile(t *testing.T) {
	s, dir := openFixture(t)
	if err := os.Remove(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "01-second.md")); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 3, "missing prose file")
}

func TestValidateDirWithoutIndexEntry(t *testing.T) {
	s, dir := openFixture(t)
	if err := os.MkdirAll(filepath.Join(dir, "changes", "2026-09-11-abcde", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 6, "no index entry")
}

func TestValidateBrokenState(t *testing.T) {
	s, dir := openFixture(t)
	p := filepath.Join(dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")
	if err := os.WriteFile(p, []byte("{garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 5, "")
	// Writes against the broken state are refused.
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err == nil || !strings.Contains(err.Error(), ErrInvalid.Error()) {
		t.Fatalf("MoveTask on broken state: err = %v", err)
	}
}

func assertViolation(t *testing.T, vs []Violation, rule int, substr string) {
	t.Helper()
	for _, v := range vs {
		if v.Rule == rule && strings.Contains(v.Msg+v.File, substr) {
			return
		}
	}
	t.Fatalf("no violation with rule %d containing %q in %v", rule, substr, vs)
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func loadState(t *testing.T, dir, id string) *model.ChangeState {
	t.Helper()
	st, err := model.LoadChangeState(filepath.Join(dir, StateDirName, "workflow", "changes", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestMoveTask(t *testing.T) {
	s, dir := openFixture(t)
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	st := loadState(t, dir, "2026-09-10-0")
	ts := st.Task("FIX-00")
	if ts == nil || ts.Status != model.StatusDone || ts.Updated != today() {
		t.Fatalf("task = %+v", ts)
	}
	// Priority order kept the moved task inside its group; a status with
	// no existing group lands at the end of the level (legacy
	// insertionPos semantics).
	if len(st.Tasks) != 2 || st.Tasks[1].ID != "FIX-00" {
		t.Fatalf("order = %+v", st.Tasks)
	}
	// Cache was updated too.
	c, _ := s.Change("2026-09-10-0")
	if c.Node("FIX-00").NodeStatus() != model.StatusDone {
		t.Fatal("cache not updated")
	}
	// Derived overall status converged (a Done task with another In
	// progress stays In progress).
	if c.Overall() != model.OverallInProgress {
		t.Fatalf("overall = %q", c.Overall())
	}
}

func TestMoveTaskPositionWithinGroup(t *testing.T) {
	s, dir := openFixture(t)
	// Three tasks: FIX-00 Not started, FIX-01 In progress, FIX-02 Not started.
	if _, err := s.CreateTask("2026-09-10-0", "", "Third"); err != nil {
		t.Fatal(err)
	}
	// Move FIX-02 to Not started index 0: it must land before FIX-00.
	if err := s.MoveTask("2026-09-10-0", "FIX-02", model.StatusNotStarted, 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	st := loadState(t, dir, "2026-09-10-0")
	if st.Tasks[0].ID != "FIX-02" || st.Tasks[1].ID != "FIX-00" {
		t.Fatalf("order after move = [%s, %s, %s], want FIX-02, FIX-00, FIX-01",
			st.Tasks[0].ID, st.Tasks[1].ID, st.Tasks[2].ID)
	}
}

func TestMoveTaskPreservesExternalEdits(t *testing.T) {
	s, dir := openFixture(t)
	// External agent appends a task directly to the state file on disk.
	ext := loadState(t, dir, "2026-09-10-0")
	ext.Tasks = append(ext.Tasks, model.TaskState{ID: "FIX-05", Seq: 5, Title: "External", File: "tasks/05-ext.md", Status: model.StatusNotStarted, Updated: "2026-09-10"})
	if err := os.WriteFile(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "05-ext.md"), model.RenderTaskFile("FIX-05", "External"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ext.Save(filepath.Join(dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")); err != nil {
		t.Fatal(err)
	}
	// A store write applies on top of the fresh content.
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	st := loadState(t, dir, "2026-09-10-0")
	if st.Task("FIX-05") == nil {
		t.Fatal("external edit lost")
	}
	if st.Task("FIX-00").Status != model.StatusDone {
		t.Fatal("move not applied")
	}
}

func TestMoveTaskUnknown(t *testing.T) {
	s, _ := openFixture(t)
	if err := s.MoveTask("2026-09-10-0", "FIX-99", model.StatusDone, 0); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := s.MoveTask("../etc", "FIX-00", model.StatusDone, 0); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateTask(t *testing.T) {
	s, dir := openFixture(t)
	row, err := s.CreateTask("2026-09-10-0", "", "My New Task!")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if row.ID != "FIX-02" || row.File != "tasks/02-my-new-task.md" || row.Status != model.StatusNotStarted {
		t.Fatalf("task = %+v", row)
	}
	// Prose file exists with the heading carrying the identity.
	body := string(mustRead(t, filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "02-my-new-task.md")))
	if !strings.HasPrefix(body, "# FIX-02: My New Task!") {
		t.Fatalf("prose file = %q", body)
	}
	// State has the task and everything still validates.
	st := loadState(t, dir, "2026-09-10-0")
	if st.Task("FIX-02") == nil {
		t.Fatal("task not in state")
	}
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after create: %v", v)
	}
}

func TestCreateTaskNoReuse(t *testing.T) {
	s, dir := openFixture(t)
	// Simulate a removed task: a gap in the sequence.
	st := loadState(t, dir, "2026-09-10-0")
	st.Tasks = append(st.Tasks, model.TaskState{ID: "FIX-05", Seq: 5, Title: "Late", File: "tasks/05-late.md", Status: model.StatusNotStarted, Updated: "2026-09-10"})
	if err := os.WriteFile(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "05-late.md"), model.RenderTaskFile("FIX-05", "Late"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(filepath.Join(dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	row, err := s.CreateTask("2026-09-10-0", "", "Another")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if row.ID != "FIX-06" {
		t.Fatalf("row.ID = %q, want FIX-06 (highest + 1, no gap reuse)", row.ID)
	}
}

func TestCreateChange(t *testing.T) {
	s, dir := openFixture(t)
	id, err := s.CreateChange("New objective", "NEW", "feat/settings", "2026-09-12")
	if err != nil {
		t.Fatalf("CreateChange: %v", err)
	}
	if !regexp.MustCompile(`^2026-09-12-[a-z0-9]{5}$`).MatchString(id) {
		t.Fatalf("id = %q, want a 2026-09-12 date prefix with a five-character lowercase alphanumeric suffix", id)
	}
	for _, f := range []string{"plan.md", "tasks"} {
		if _, err := os.Stat(filepath.Join(dir, "changes", id, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
	idx, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}
	e := idx.Find(id)
	if e == nil || e.Prefix != "NEW" || e.Branch != "feat/settings" {
		t.Fatalf("index entry = %+v", e)
	}
	st := loadState(t, dir, id)
	if st.Status != (model.ChangeStatus{Value: model.OverallPlanned, Derived: true}) {
		t.Fatalf("new change status = %+v", st.Status)
	}
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after create: %v", v)
	}
}

func TestCreateChangeEmptyBranch(t *testing.T) {
	s, dir := openFixture(t)
	id, err := s.CreateChange("Plain", "PLA", "", "2026-09-12")
	if err != nil {
		t.Fatalf("CreateChange: %v", err)
	}
	st := loadState(t, dir, id)
	if st.Branch != "" {
		t.Fatalf("branch = %q, want empty", st.Branch)
	}
}

func TestCreateChangeSuffixUniqueness(t *testing.T) {
	s, _ := openFixture(t)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id, err := s.CreateChange("Objective", "", "", "2026-09-12")
		if err != nil {
			t.Fatalf("CreateChange %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q minted", id)
		}
		seen[id] = true
	}
}

func TestCreateChangeCollisionRegenerates(t *testing.T) {
	s, dir := openFixture(t)
	// Pre-occupy the suffix the stubbed source returns first; CreateChange
	// must regenerate instead of colliding or failing.
	occupied := filepath.Join(dir, "changes", "2026-09-12-abcde", "tasks")
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatal(err)
	}
	orig := randSuffix
	defer func() { randSuffix = orig }()
	suffixes := []string{"abcde", "zz9x9"}
	randSuffix = func() string {
		next := suffixes[0]
		if len(suffixes) > 1 {
			suffixes = suffixes[1:]
		}
		return next
	}
	id, err := s.CreateChange("After collision", "", "", "2026-09-12")
	if err != nil {
		t.Fatalf("CreateChange: %v", err)
	}
	if id != "2026-09-12-zz9x9" {
		t.Fatalf("id = %q, want 2026-09-12-zz9x9 (second candidate)", id)
	}
}

func TestCreateChangeSuffixExhausted(t *testing.T) {
	s, dir := openFixture(t)
	occupied := filepath.Join(dir, "changes", "2026-09-12-abcde", "tasks")
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatal(err)
	}
	orig := randSuffix
	defer func() { randSuffix = orig }()
	randSuffix = func() string { return "abcde" }
	if _, err := s.CreateChange("Doomed", "", "", "2026-09-12"); err == nil {
		t.Fatal("expected an error when every mint collides")
	}
}

func TestValidChangeDirName(t *testing.T) {
	valid := []string{
		"2026-09-10-0",     // legacy numeric
		"2026-09-10-12",    // legacy numeric, multi-digit
		"2026-09-10-a1b2c", // new five-char lowercase alphanumeric
		"2026-09-10-00012", // five digits (matches both forms)
	}
	for _, name := range valid {
		if !validChangeDirName(name) {
			t.Errorf("validChangeDirName(%q) = false, want true", name)
		}
	}
	invalid := []string{
		"bogus",
		"2026-09-10",
		"2026-09-10-",
		"2026-09-10-ABCDE",  // uppercase
		"2026-09-10-abcd",   // four chars
		"2026-09-10-abcdef", // six chars
		"2026-13-40-abcde",  // invalid date
		"2026-09-10-ab1",    // three chars
	}
	for _, name := range invalid {
		if validChangeDirName(name) {
			t.Errorf("validChangeDirName(%q) = true, want false", name)
		}
	}
}

func TestSetChangeStatus(t *testing.T) {
	s, dir := openFixture(t)

	if err := s.SetChangeStatus("2026-09-10-0", model.OverallDone); err != nil {
		t.Fatalf("close: %v", err)
	}
	st := loadState(t, dir, "2026-09-10-0")
	if st.Status != (model.ChangeStatus{Value: model.OverallDone, Derived: false}) || st.Updated != today() {
		t.Fatalf("state after close = %+v", st.Status)
	}
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after close: %v", v)
	}

	// Reopen round trip.
	if err := s.SetChangeStatus("2026-09-10-0", model.OverallInProgress); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	st = loadState(t, dir, "2026-09-10-0")
	if st.Status != (model.ChangeStatus{Value: model.OverallInProgress, Derived: true}) {
		t.Fatalf("state after reopen = %+v", st.Status)
	}

	// Unknown change.
	if err := s.SetChangeStatus("2099-01-01-9", model.OverallDone); err != ErrNotFound {
		t.Fatalf("unknown: err = %v", err)
	}
}

func TestSyncOverallConvergesDrift(t *testing.T) {
	s, dir := openFixture(t)
	// All tasks done/test: the tree is complete, so the derived status is
	// In progress (close is user-gated).
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusTest, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask("2026-09-10-0", "FIX-01", model.StatusDone, 0); err != nil {
		t.Fatal(err)
	}
	st := loadState(t, dir, "2026-09-10-0")
	if st.Status.Value != model.OverallInProgress || !st.Status.Derived {
		t.Fatalf("status = %+v, want derived In progress", st.Status)
	}
	// All tasks Not started again (external edit): the watcher's
	// SyncOverallStatuses converges back to Planned.
	st.Tasks[0].Status = model.StatusNotStarted
	st.Tasks[1].Status = model.StatusNotStarted
	if err := st.Save(filepath.Join(dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	s.SyncOverallStatuses()
	st = loadState(t, dir, "2026-09-10-0")
	if st.Status.Value != model.OverallPlanned {
		t.Fatalf("status after converge = %+v", st.Status)
	}
	// A user-set Done survives recomputation.
	if err := s.SetChangeStatus("2026-09-10-0", model.OverallDone); err != nil {
		t.Fatal(err)
	}
	s.SyncOverallStatuses()
	st = loadState(t, dir, "2026-09-10-0")
	if st.Status != (model.ChangeStatus{Value: model.OverallDone, Derived: false}) {
		t.Fatalf("user-set status clobbered: %+v", st.Status)
	}
}

func TestWatchEvent(t *testing.T) {
	s, dir := openFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := s.Watch(ctx); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	ch := s.Subscribe()
	// External edit to the state file.
	p := filepath.Join(dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")
	st := loadState(t, dir, "2026-09-10-0")
	st.Tasks[0].Notes = "watched"
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if ev.Kind != "fs" {
			t.Fatalf("event kind = %q", ev.Kind)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no fs event within 5s")
	}
}

func TestWatchIgnoresChmod(t *testing.T) {
	s, dir := openFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := s.Watch(ctx); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	ch := s.Subscribe()
	// Attribute-only touches (chmod, and atime updates from readers such as
	// git status, which surface as Chmod on macOS) must not notify —
	// otherwise read-heavy scans retrigger clients forever.
	p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first.md")
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		t.Fatalf("chmod produced unexpected %q event", ev.Kind)
	case <-time.After(1 * time.Second):
	}
	// A real content write still notifies.
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\nreal edit\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	select {
	case ev := <-ch:
		if ev.Kind != "fs" {
			t.Fatalf("event kind = %q", ev.Kind)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no fs event within 5s after real write")
	}
}

func TestWriteNotifies(t *testing.T) {
	s, _ := openFixture(t)
	ch := s.Subscribe()
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if ev.Kind != "write" {
			t.Fatalf("kind = %q", ev.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("no write event")
	}
}

func TestOpenNoChangesSentinel(t *testing.T) {
	_, err := Open(t.TempDir())
	if !errors.Is(err, ErrNoChanges) {
		t.Fatalf("Open err = %v, want errors.Is ErrNoChanges", err)
	}
	if !strings.Contains(err.Error(), "workflow state not found under") {
		t.Fatalf("message changed: %v", err)
	}
}

func TestOpenMissingIndexSentinel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Open(dir)
	if !errors.Is(err, ErrNoChanges) {
		t.Fatalf("partial tree Open err = %v, want errors.Is ErrNoChanges", err)
	}
}

func TestOpenLegacyMarkdownNotSentinel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "changes", "ledger.md"), []byte("legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(dir)
	if !errors.Is(err, ErrLegacyMarkdown) {
		t.Fatalf("legacy Open err = %v, want errors.Is ErrLegacyMarkdown", err)
	}
	if errors.Is(err, ErrNoChanges) {
		t.Fatalf("legacy tree must NOT be ErrNoChanges: %v", err)
	}
}

func TestOpenCorruptIndexNotSentinel(t *testing.T) {
	dir := t.TempDir()
	wd := filepath.Join(dir, StateDirName, "workflow", "changes")
	if err := os.MkdirAll(wd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, StateDirName, "workflow", "index.json"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(dir)
	if err == nil {
		t.Fatal("corrupt index must fail")
	}
	if errors.Is(err, ErrNoChanges) {
		t.Fatalf("corrupt index must NOT be ErrNoChanges (stays fatal): %v", err)
	}
}

func TestTaskAndPlanReaders(t *testing.T) {
	s, _ := openFixture(t)
	tf, err := s.TaskFile("2026-09-10-0", "tasks/00-first.md")
	if err != nil {
		t.Fatalf("TaskFile: %v", err)
	}
	if tf.ID != "FIX-00" || tf.Title != "First" || !strings.Contains(tf.Body, "# FIX-00: First") {
		t.Fatalf("task file = %+v", tf)
	}
	// Identity comes from state: an unreferenced href is not a task.
	if _, err := s.TaskFile("2026-09-10-0", "tasks/99-ghost.md"); err != ErrNotFound {
		t.Fatalf("ghost href err = %v, want ErrNotFound", err)
	}
	// Path guards.
	if _, err := s.TaskFile("2026-09-10-0", "../escape.md"); err != ErrNotFound {
		t.Fatalf("escape err = %v", err)
	}
	plan, err := s.PlanFile("2026-09-10-0")
	if err != nil || !strings.Contains(plan, "Fixture change") {
		t.Fatalf("PlanFile: %v %q", err, plan)
	}
}

func TestLedgerViews(t *testing.T) {
	s, _ := openFixture(t)
	view, err := s.LedgerFile("2026-09-10-0")
	if err != nil {
		t.Fatalf("LedgerFile: %v", err)
	}
	for _, want := range []string{
		"- Overall status: In progress",
		"[FIX-00](tasks/00-first.md)",
		"[FIX-01](tasks/01-second.md)",
		"| Task | Title | Status | Depends on | Updated | Notes |",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	// Container view: decompose and add a child, then fetch the legacy
	// href shape.
	if _, err := s.DecomposeTask("2026-09-10-0", "FIX-00"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child"); err != nil {
		t.Fatal(err)
	}
	cv, err := s.ContainerLedgerFile("2026-09-10-0", "tasks/00-first/ledger.md")
	if err != nil {
		t.Fatalf("ContainerLedgerFile: %v", err)
	}
	if !strings.Contains(cv, "[FIX-00.00](tasks/00-first/tasks/00-child.md)") {
		t.Errorf("container view = %s", cv)
	}
	if _, err := s.ContainerLedgerFile("2026-09-10-0", "plan.md"); err != ErrNotFound {
		t.Errorf("non-container href err = %v, want ErrNotFound", err)
	}
}
