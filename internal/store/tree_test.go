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
	// The container prose directory exists; no container ledger does.
	if _, err := os.Stat(filepath.Join(cdir, "tasks", "00-first", "tasks")); err != nil {
		t.Fatalf("missing container tasks dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cdir, "tasks", "00-first", "ledger.md")); !os.IsNotExist(err) {
		t.Error("container ledger.md must not exist in the JSON model")
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
	if row.ID != "FIX-00.00" || row.Parent != "FIX-00" {
		t.Errorf("child = %+v, want FIX-00.00 under FIX-00", row)
	}
	cdir := filepath.Join(dir, "changes", "2026-09-10-0")
	if _, err := os.Stat(filepath.Join(cdir, "tasks", "00-first", "tasks", "00-child-one.md")); err != nil {
		t.Fatalf("child file: %v", err)
	}
	// The task landed in the state with the right href.
	st := loadState(t, dir, "2026-09-10-0")
	child := st.Task("FIX-00.00")
	if child == nil || child.File != "tasks/00-first/tasks/00-child-one.md" || child.Seq != 0 {
		t.Fatalf("child state = %+v", child)
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
	s, _ := openFixture(t)
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

	// A stray directory (no referenced prose inside) is recorded for
	// validation; the container is not.
	if err := os.MkdirAll(filepath.Join(s.Dir, "changes", "2026-09-10-0", "tasks", "99-stray"), 0o755); err != nil {
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

func TestMoveSubtaskAtOwnLevel(t *testing.T) {
	s, _ := openFixture(t)
	decompose(t, s)
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child One"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00", "Child Two"); err != nil {
		t.Fatal(err)
	}

	// Move the first child to Test at index 0 of its group; the parent's
	// status and the sibling's position are untouched.
	if err := s.MoveTask("2026-09-10-0", "FIX-00.00", model.StatusTest, 0); err != nil {
		t.Fatalf("MoveTask child: %v", err)
	}
	c, _ := s.Change("2026-09-10-0")
	if got := c.Node("FIX-00.00").NodeStatus(); got != model.StatusTest {
		t.Fatalf("child status after move = %q", got)
	}
	if got := c.Node("FIX-00").NodeStatus(); got != model.StatusNotStarted {
		t.Fatalf("parent status changed: %q", got)
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

	decompose(t, s) // creates tasks/00-first/tasks/
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("no fs event after decompose")
	}

	// An external state edit adding a nested task is picked up too: the
	// watcher re-walks after each quiet period.
	p := filepath.Join(s.Dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")
	st := loadState(t, s.Dir, "2026-09-10-0")
	st.Tasks = append(st.Tasks, model.TaskState{
		ID: "FIX-00.00", Seq: 0, Parent: "FIX-00", Title: "Child",
		File: "tasks/00-first/tasks/00-child.md", Status: model.StatusNotStarted, Updated: "2026-09-16",
	})
	if err := os.MkdirAll(filepath.Join(s.Dir, "changes", "2026-09-10-0", "tasks", "00-first", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "changes", "2026-09-10-0", "tasks", "00-first", "tasks", "00-child.md"), model.RenderTaskFile("FIX-00.00", "Child"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(p); err != nil {
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
	if n := c.Node("FIX-00.00"); n == nil || n.Href != "tasks/00-first/tasks/00-child.md" {
		t.Fatalf("nested node after watch reload = %+v", c.Node("FIX-00.00"))
	}
}
