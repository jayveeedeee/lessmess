package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lessmess/internal/model"
)

// decompose is a helper: decompose FIX-00 in the fixture change.
func decompose(t *testing.T, s *Store) string {
	t.Helper()
	rel, err := s.DecomposeTask("2026-09-10-0", "FIX-00")
	if err != nil {
		t.Fatalf("DecomposeTask: %v", err)
	}
	return rel
}

func TestDecomposeTask(t *testing.T) {
	s, dir := openFixture(t)
	rel := decompose(t, s)
	if rel != "tasks/00-first" {
		t.Errorf("container rel = %q", rel)
	}
	cdir := filepath.Join(dir, "changes", "2026-09-10-0")
	for _, p := range []string{"tasks/00-first/ledger.md", "tasks/00-first/tasks"} {
		if st, err := os.Stat(filepath.Join(cdir, filepath.FromSlash(p))); err != nil || (p == "tasks" && !st.IsDir()) {
			t.Errorf("missing %s (%v)", p, err)
		}
	}
	// The container ledger parses and declares the right task/change.
	tl, err := model.ParseTaskLedger("ledger.md", readFile(t, filepath.Join(cdir, "tasks", "00-first", "ledger.md")))
	if err != nil {
		t.Fatalf("parse container ledger: %v", err)
	}
	if tl.TaskID != "FIX-00" || tl.ChangeID != "2026-09-10-0" || len(tl.Rows) != 0 {
		t.Errorf("container ledger = %+v", tl)
	}
	// Second decompose is refused.
	if _, err := s.DecomposeTask("2026-09-10-0", "FIX-00"); !errors.Is(err, ErrContainerExists) {
		t.Errorf("second decompose err = %v, want ErrContainerExists", err)
	}
	// Unknown tasks and changes are refused.
	if _, err := s.DecomposeTask("2026-09-10-0", "FIX-99"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown task err = %v, want ErrNotFound", err)
	}
	if _, err := s.DecomposeTask("2026-09-10-9", "FIX-00"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown change err = %v, want ErrNotFound", err)
	}
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCreateSubtask(t *testing.T) {
	s, dir := openFixture(t)
	decompose(t, s)

	row, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child One!")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if row.ID != "FIX-00.00" {
		t.Errorf("child ID = %q, want FIX-00.00", row.ID)
	}
	cdir := filepath.Join(dir, "changes", "2026-09-10-0")
	if _, err := os.Stat(filepath.Join(cdir, "tasks", "00-first", "tasks", "00-child-one.md")); err != nil {
		t.Fatalf("child file: %v", err)
	}
	// The row landed in the container ledger with a ledger-relative href.
	tl, err := model.ParseTaskLedger("ledger.md", readFile(t, filepath.Join(cdir, "tasks", "00-first", "ledger.md")))
	if err != nil {
		t.Fatalf("parse container ledger: %v", err)
	}
	if len(tl.Rows) != 1 || tl.Rows[0].ID != "FIX-00.00" || tl.Rows[0].Href != "tasks/00-child-one.md" {
		t.Fatalf("container rows = %+v", tl.Rows)
	}

	// Second child gets sequence 01.
	row2, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child Two")
	if err != nil {
		t.Fatalf("CreateTask 2: %v", err)
	}
	if row2.ID != "FIX-00.01" {
		t.Errorf("second child ID = %q, want FIX-00.01", row2.ID)
	}

	// Subtasks under a non-decomposed task are refused.
	if _, err := s.CreateTask("2026-09-10-0", "FIX-01", "Nope"); !errors.Is(err, ErrNoContainer) {
		t.Errorf("non-decomposed parent err = %v, want ErrNoContainer", err)
	}
	// Unknown parent refused.
	if _, err := s.CreateTask("2026-09-10-0", "FIX-99", "Nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown parent err = %v, want ErrNotFound", err)
	}
}

func TestScanNestedTree(t *testing.T) {
	s, dir := openFixture(t)
	decompose(t, s)
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child One"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child Two"); err != nil {
		t.Fatal(err)
	}
	// Grandchild level: decompose the first child.
	if _, err := s.DecomposeTask("2026-09-10-0", "FIX-00.00"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00.00", "Grandchild"); err != nil {
		t.Fatal(err)
	}

	c, err := s.Change("2026-09-10-0")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Roots) != 2 {
		t.Fatalf("roots = %d, want 2", len(c.Roots))
	}
	top := c.Node("FIX-00")
	if top == nil || top.Parent != nil || len(top.Children) != 2 {
		t.Fatalf("FIX-00 node = %+v children=%d", top, len(top.Children))
	}
	child0 := c.Node("FIX-00.00")
	if child0 == nil || child0.Parent != top {
		t.Fatalf("FIX-00.00 parent = %+v", child0.Parent)
	}
	if child0.Href != "tasks/00-first/tasks/00-child-one.md" {
		t.Errorf("child0 href = %q", child0.Href)
	}
	grand := c.Node("FIX-00.00.00")
	if grand == nil || grand.Parent != child0 || grand.Href != "tasks/00-first/tasks/00-child-one/tasks/00-grandchild.md" {
		t.Fatalf("grand node = %+v", grand)
	}
	if !c.Node("FIX-00").HasContainer() || c.Node("FIX-01").HasContainer() {
		t.Error("HasContainer wrong")
	}

	// A stray directory (no sibling task file) is recorded for validation.
	if err := os.MkdirAll(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "99-stray"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	c, _ = s.Change("2026-09-10-0")
	found := false
	for _, d := range c.StrayDirs {
		if d == "tasks/99-stray" {
			found = true
		}
		if d == "tasks/00-first" {
			t.Error("container tasks/00-first falsely flagged stray")
		}
	}
	if !found {
		t.Errorf("StrayDirs = %v, want tasks/99-stray", c.StrayDirs)
	}
}

func TestMoveSubtaskViaGoverningLedger(t *testing.T) {
	s, dir := openFixture(t)
	decompose(t, s)
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child One"); err != nil {
		t.Fatal(err)
	}

	// Move the child; the container ledger changes, the change ledger does not.
	changeLedgerBefore := readFile(t, filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md"))
	if err := s.MoveTask("2026-09-10-0", "FIX-00.00", model.StatusTest, 0); err != nil {
		t.Fatalf("MoveTask child: %v", err)
	}
	tl, err := model.ParseTaskLedger("ledger.md", readFile(t, filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first", "ledger.md")))
	if err != nil {
		t.Fatal(err)
	}
	if row := tl.Row("FIX-00.00"); row == nil || row.Status != model.StatusTest {
		t.Fatalf("child row after move = %+v", row)
	}
	if got := readFile(t, filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")); string(got) != string(changeLedgerBefore) {
		t.Error("change ledger changed by child move")
	}
	// Unknown child ID refused.
	if err := s.MoveTask("2026-09-10-0", "FIX-00.99", model.StatusDone, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown child err = %v, want ErrNotFound", err)
	}
}

func TestSubtreeStats(t *testing.T) {
	s, _ := openFixture(t)
	decompose(t, s)
	setStatus := func(id string, st model.TaskStatus) {
		t.Helper()
		if err := s.MoveTask("2026-09-10-0", id, st, 0); err != nil {
			t.Fatalf("move %s: %v", id, err)
		}
	}
	for _, title := range []string{"A", "B", "C"} {
		if _, err := s.CreateTask("2026-09-10-0", "FIX-00", title); err != nil {
			t.Fatal(err)
		}
	}
	setStatus("FIX-00.00", model.StatusDone)
	setStatus("FIX-00.01", model.StatusTest)
	setStatus("FIX-00.02", model.StatusCancelled)

	// Grandchild level: FIX-00.00 gets its own subtree.
	if _, err := s.DecomposeTask("2026-09-10-0", "FIX-00.00"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00.00", "Grand"); err != nil {
		t.Fatal(err)
	}
	setStatus("FIX-00.00.00", model.StatusDone)

	c, err := s.Change("2026-09-10-0")
	if err != nil {
		t.Fatal(err)
	}
	st := c.Node("FIX-00").SubtreeStats()
	if st.Total != 3 || st.Complete != 3 {
		t.Errorf("FIX-00 stats = %+v, want Total=3 Complete=3 (2 children + 1 grandchild, cancelled excluded)", st)
	}
	if st.ByStatus[model.StatusDone] != 2 || st.ByStatus[model.StatusTest] != 1 || st.ByStatus[model.StatusCancelled] != 1 {
		t.Errorf("ByStatus = %v", st.ByStatus)
	}
	// Change-wide stats count every node, roots included: FIX-00 (Not
	// started), FIX-01 (In progress), 3 children (1 cancelled), 1 grandchild.
	all := c.AllTaskStats()
	if all.Total != 5 || all.Complete != 3 {
		t.Errorf("AllTaskStats = %+v, want Total=5 Complete=3 (all non-cancelled nodes)", all)
	}
}

func TestWatchPicksUpNestedDirs(t *testing.T) {
	s, _ := openFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := s.Watch(ctx); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	ch := s.Subscribe()

	decompose(t, s) // creates tasks/00-first/{ledger.md,tasks/}
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("no fs event after decompose")
	}

	// A grandchild level created by raw file writes is picked up too:
	// the watcher re-walks after each quiet period, so newly created
	// nested directories stay watched.
	cdir := filepath.Join(s.Dir, "changes", "2026-09-10-0", "tasks", "00-first")
	if err := os.MkdirAll(filepath.Join(cdir, "tasks", "00-child", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "tasks", "00-child.md"), model.RenderTaskFile("FIX-00.00", "Child"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "ledger.md"), model.RenderTaskLedger("FIX-00", "2026-09-10-0", "2026-09-16"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("no fs event after nested creation")
	}
	c, err := s.Change("2026-09-10-0")
	if err != nil {
		t.Fatal(err)
	}
	if n := c.Node("FIX-00.00"); n == nil || !n.HasContainer() {
		t.Fatalf("nested node after watch reload = %+v", c.Node("FIX-00.00"))
	}
}
