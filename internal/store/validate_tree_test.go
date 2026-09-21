package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// nestedFixture opens the fixture change with FIX-00 decomposed into two
// children, the first of which is itself decomposed with one grandchild.
func nestedFixture(t *testing.T) (*Store, string) {
	t.Helper()
	s, dir := openFixture(t)
	decompose(t, s)
	for _, title := range []string{"Child One", "Child Two"} {
		if _, err := s.CreateTask("2026-09-10-0", "FIX-00", title); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DecomposeTask("2026-09-10-0", "FIX-00.00"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask("2026-09-10-0", "FIX-00.00", "Grand"); err != nil {
		t.Fatal(err)
	}
	v := s.Validate()
	if len(v) != 0 {
		t.Fatalf("baseline violations: %v", v)
	}
	return s, dir
}

func hasViolation(t *testing.T, v []Violation, rule int, file, substr string) {
	t.Helper()
	for _, x := range v {
		if x.Rule == rule && strings.Contains(x.File, file) && strings.Contains(x.Msg, substr) {
			return
		}
	}
	t.Fatalf("no rule %d violation on %q containing %q in %v", rule, file, substr, v)
}

func TestValidateCleanNestedTree(t *testing.T) {
	s, _ := nestedFixture(t)
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations = %v", v)
	}
}

func TestValidateStrayDir(t *testing.T) {
	s, dir := nestedFixture(t)
	if err := os.MkdirAll(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "77-stray"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	hasViolation(t, s.Validate(), 3, "tasks/77-stray", "not part of any sub plan")
}

// mutateState applies f to the fixture change's state on disk and reloads.
func mutateState(t *testing.T, s *Store, f func(*model.ChangeState)) {
	t.Helper()
	p := filepath.Join(s.Dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")
	st, err := model.LoadChangeState(p)
	if err != nil {
		t.Fatal(err)
	}
	f(st)
	if err := os.WriteFile(p, mustRead(t, p)[:0], 0o644); err != nil { // truncate in place
		t.Fatal(err)
	}
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
	s.Reload()
}

func TestValidateMissingNestedProse(t *testing.T) {
	s, dir := nestedFixture(t)
	grand := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first", "tasks", "00-child-one", "tasks", "00-grand.md")
	if err := os.Remove(grand); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	hasViolation(t, s.Validate(), 3, "00-grand.md", "missing prose file")
}

func TestValidateNestedSequenceAndDepth(t *testing.T) {
	s, _ := nestedFixture(t)

	// Sequence disagreement between a task's id and its file name.
	mutateState(t, s, func(st *model.ChangeState) {
		st.Task("FIX-00.00.00").File = "tasks/00-first/tasks/00-child-one/tasks/05-grand.md"
	})
	hasViolation(t, s.Validate(), 5, "changes/2026-09-10-0", "does not start with seq 00")

	// Restore, then make the id one level too deep for its nesting.
	mutateState(t, s, func(st *model.ChangeState) {
		g := st.Task("FIX-00.00.00")
		g.File = "tasks/00-first/tasks/00-child-one/tasks/00-grand.md"
		g.ID = "FIX-00.00.00.09"
		g.Parent = "FIX-00.00.00" // keep parent coherent so depth is the flagged issue
	})
	hasViolation(t, s.Validate(), 5, "changes/2026-09-10-0", "parent")

	// A single-digit dotted segment is malformed.
	mutateState(t, s, func(st *model.ChangeState) {
		g := st.Task("FIX-00.00.00.09")
		g.ID = "FIX-00.00.0"
		g.Parent = "FIX-00.00"
	})
	hasViolation(t, s.Validate(), 5, "changes/2026-09-10-0", "two-digit")
}

func TestValidateOrphanChildFile(t *testing.T) {
	s, _ := nestedFixture(t)
	// Drop the second child from the state; its prose file becomes an
	// orphan.
	mutateState(t, s, func(st *model.ChangeState) {
		var tasks []model.TaskState
		for i := range st.Tasks {
			if st.Tasks[i].ID != "FIX-00.01" {
				tasks = append(tasks, st.Tasks[i])
			}
		}
		st.Tasks = tasks
	})
	hasViolation(t, s.Validate(), 3, "01-child-two.md", "not referenced")
}

func TestCloseOutReadyRecursive(t *testing.T) {
	s, _ := nestedFixture(t)
	// Grandchild open blocks close-out.
	ok, offending, err := s.CloseOutReady("2026-09-10-0")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("close-out ready with everything Not started")
	}
	if len(offending) == 0 {
		t.Fatal("offending list empty")
	}

	// Every node in the tree to Test (deepest first for clarity).
	for _, id := range []string{"FIX-00.00.00", "FIX-00.00", "FIX-00.01", "FIX-00", "FIX-01"} {
		if err := s.MoveTask("2026-09-10-0", id, model.StatusTest, 0); err != nil {
			t.Fatalf("move %s: %v", id, err)
		}
	}
	ok, offending, _ = s.CloseOutReady("2026-09-10-0")
	if !ok || len(offending) != 0 {
		t.Fatalf("close-out not ready: %v", offending)
	}

	// Cancelled tasks leave the denominator: cancel FIX-01 and stay ready.
	if err := s.MoveTask("2026-09-10-0", "FIX-01", model.StatusCancelled, 0); err != nil {
		t.Fatal(err)
	}
	ok, _, _ = s.CloseOutReady("2026-09-10-0")
	if !ok {
		t.Fatal("cancelled task blocks close-out")
	}
}
