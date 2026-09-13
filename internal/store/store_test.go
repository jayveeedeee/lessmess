package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lessmess/internal/model"
)

// writeFixture builds a minimal valid changes/ tree in dir.
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

	root := `# Changes — Root Ledger

One row per change directory. Task statuses live exclusively in each change's ledger.

| Change | Title | ID prefix | Branch | Status | Created | Last updated |
| --- | --- | --- | --- | --- | --- | --- |
| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | In progress | 2026-09-10 | 2026-09-10 |
`
	must(os.WriteFile(filepath.Join(dir, "changes", "ledger.md"), []byte(root), 0o644))
	must(os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan("2026-09-10-0", "Fixture change", "2026-09-10"), 0o644))

	ledger := model.RenderChangeLedger("2026-09-10-0", "2026-09-10")
	l, err := model.ParseChangeLedger("ledger.md", ledger)
	if err != nil {
		t.Fatal(err)
	}
	l.SetOverall(model.OverallInProgress, "2026-09-10")
	l.AppendTask(model.TaskRow{ID: "FIX-00", Href: "tasks/00-first.md", Title: "First", Status: model.StatusNotStarted, Updated: "2026-09-10", Notes: model.Empty})
	l.AppendTask(model.TaskRow{ID: "FIX-01", Href: "tasks/01-second.md", Title: "Second", Status: model.StatusInProgress, Updated: "2026-09-10", Notes: model.Empty})
	must(os.WriteFile(filepath.Join(cdir, "ledger.md"), l.Content(), 0o644))
	must(os.WriteFile(filepath.Join(cdir, "tasks", "00-first.md"), model.RenderTaskFile("FIX-00", "First"), 0o644))
	must(os.WriteFile(filepath.Join(cdir, "tasks", "01-second.md"), model.RenderTaskFile("FIX-01", "Second"), 0o644))
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
	if len(c.Ledger.Rows) != 2 || c.Ledger.Rows[0].ID != "FIX-00" {
		t.Fatalf("rows = %+v", c.Ledger.Rows)
	}
	if len(c.Tasks) != 2 || c.Tasks["tasks/00-first.md"].Title != "First" {
		t.Fatalf("tasks = %+v", c.Tasks)
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

func TestValidateMissingFile(t *testing.T) {
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
	if err := os.WriteFile(p, model.RenderTaskFile("FIX-02", "Orphan"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 3, "no ledger row")
}

func TestValidateRowWithoutFile(t *testing.T) {
	s, dir := openFixture(t)
	if err := os.Remove(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "01-second.md")); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 3, "no parseable task file")
}

func TestValidateIDFilenameMismatch(t *testing.T) {
	s, dir := openFixture(t)
	p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "01-second.md")
	if err := os.WriteFile(p, model.RenderTaskFile("FIX-09", "Second"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	// Ledger row FIX-01, filename 01, frontmatter FIX-09: the row↔filename
	// pair agrees; only the frontmatter mismatches.
	assertViolation(t, s.Validate(), 3, "frontmatter id")
}

func TestValidateRootStatusMismatch(t *testing.T) {
	s, dir := openFixture(t)
	p := filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")
	l, _ := model.ParseChangeLedger("l", mustRead(t, p))
	l.SetOverall(model.OverallDone, "2026-09-11")
	if err := os.WriteFile(p, l.Content(), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 6, "!=")
}

func TestValidateBrokenSchema(t *testing.T) {
	s, dir := openFixture(t)
	p := filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")
	if err := os.WriteFile(p, []byte("# garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	assertViolation(t, s.Validate(), 5, "")
	// Writes to the broken ledger are refused.
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err == nil || !strings.Contains(err.Error(), ErrInvalid.Error()) {
		t.Fatalf("MoveTask on broken ledger: err = %v", err)
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

func TestMoveTask(t *testing.T) {
	s, dir := openFixture(t)
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	l, err := model.ParseChangeLedger("l", mustRead(t, filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	row := l.Row("FIX-00")
	if row == nil || row.Status != model.StatusDone || row.Updated != today() {
		t.Fatalf("row = %+v", row)
	}
	// Cache was updated too.
	c, _ := s.Change("2026-09-10-0")
	if c.Ledger.Row("FIX-00").Status != model.StatusDone {
		t.Fatal("cache not updated")
	}
}

func TestMoveTaskPreservesExternalEdits(t *testing.T) {
	s, dir := openFixture(t)
	// External agent appends a row directly on disk.
	p := filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")
	l, _ := model.ParseChangeLedger("l", mustRead(t, p))
	l.AppendTask(model.TaskRow{ID: "FIX-05", Href: "tasks/05-ext.md", Title: "External", Status: model.StatusNotStarted, Updated: "2026-09-10", Notes: model.Empty})
	if err := os.WriteFile(p, l.Content(), 0o644); err != nil {
		t.Fatal(err)
	}
	// A store write applies on top of the fresh content.
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	l2, _ := model.ParseChangeLedger("l", mustRead(t, p))
	if l2.Row("FIX-05") == nil {
		t.Fatal("external edit lost")
	}
	if l2.Row("FIX-00").Status != model.StatusDone {
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
	row, err := s.CreateTask("2026-09-10-0", "My New Task!")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if row.ID != "FIX-02" || row.Href != "tasks/02-my-new-task.md" || row.Status != model.StatusNotStarted {
		t.Fatalf("row = %+v", row)
	}
	// File exists and parses.
	tf, err := model.ParseTaskFile("t", mustRead(t, filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "02-my-new-task.md")))
	if err != nil || tf.ID != "FIX-02" {
		t.Fatalf("task file: %v %+v", err, tf)
	}
	// Ledger row appended and everything still validates.
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after create: %v", v)
	}
}

func TestCreateTaskNoReuse(t *testing.T) {
	s, dir := openFixture(t)
	// Simulate a removed task: gap between 01 and 05 on disk.
	p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "05-late.md")
	if err := os.WriteFile(p, model.RenderTaskFile("FIX-05", "Late"), 0o644); err != nil {
		t.Fatal(err)
	}
	row, err := s.CreateTask("2026-09-10-0", "Another")
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
	if id != "2026-09-12-0" {
		t.Fatalf("id = %q", id)
	}
	for _, f := range []string{"plan.md", "ledger.md", "tasks"} {
		if _, err := os.Stat(filepath.Join(dir, "changes", id, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range root.Rows {
		if r.Change == id && r.Prefix == "NEW" && r.Status == model.OverallPlanned && r.Branch == "feat/settings" {
			found = true
		}
	}
	if !found {
		t.Fatal("root row not appended with branch")
	}
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after create: %v", v)
	}
}

func TestCreateChangeEmptyBranch(t *testing.T) {
	s, _ := openFixture(t)
	id, err := s.CreateChange("Plain", "PLA", "", "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range root.Rows {
		if r.Change == id && r.Branch != model.Empty {
			t.Fatalf("branch = %q, want %q", r.Branch, model.Empty)
		}
	}
}

func TestCreateChangeGapRule(t *testing.T) {
	s, dir := openFixture(t)
	// Existing 2026-09-12-0 and -2 (gap: -1 removed) → next must be -3,
	// per the no-reuse rule in AGENTS.md.
	for _, n := range []string{"2026-09-12-0", "2026-09-12-2"} {
		if err := os.MkdirAll(filepath.Join(dir, "changes", n, "tasks"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	id, err := s.CreateChange("Third", "—", "", "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if id != "2026-09-12-3" {
		t.Fatalf("id = %q, want 2026-09-12-3 (highest + 1, gaps not reused)", id)
	}
}

func TestSetChangeStatus(t *testing.T) {
	s, dir := openFixture(t)

	if err := s.SetChangeStatus("2026-09-10-0", model.OverallDone); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Per-change ledger updated.
	l, err := model.ParseChangeLedger("l", mustRead(t, filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")))
	if err != nil {
		t.Fatal(err)
	}
	if l.Overall != model.OverallDone || l.LastUpdated != today() {
		t.Fatalf("ledger overall = %q updated = %q", l.Overall, l.LastUpdated)
	}
	// Root ledger row updated.
	root, err := model.ParseRootLedger("r", mustRead(t, filepath.Join(dir, "changes", "ledger.md")))
	if err != nil {
		t.Fatal(err)
	}
	if root.Rows[0].Status != model.OverallDone || root.Rows[0].Updated != today() {
		t.Fatalf("root row = %+v", root.Rows[0])
	}
	// Everything still validates.
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after close: %v", v)
	}

	// Reopen round trip.
	if err := s.SetChangeStatus("2026-09-10-0", model.OverallInProgress); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	l2, _ := model.ParseChangeLedger("l", mustRead(t, filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")))
	root2, _ := model.ParseRootLedger("r", mustRead(t, filepath.Join(dir, "changes", "ledger.md")))
	if l2.Overall != model.OverallInProgress || root2.Rows[0].Status != model.OverallInProgress {
		t.Fatalf("after reopen: ledger %q root %q", l2.Overall, root2.Rows[0].Status)
	}
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after reopen: %v", v)
	}

	// Unknown change.
	if err := s.SetChangeStatus("2099-01-01-9", model.OverallDone); err != ErrNotFound {
		t.Fatalf("unknown: err = %v", err)
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
	// External edit.
	p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first.md")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\nexternal edit\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
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
