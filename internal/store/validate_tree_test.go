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
	hasViolation(t, s.Validate(), 3, "tasks/77-stray", "no matching sibling task file")
}

func rewrite(t *testing.T, path string, old, new string) {
	t.Helper()
	b := strings.Replace(string(readFile(t, path)), old, new, 1)
	if err := os.WriteFile(path, []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateContainerMissingPieces(t *testing.T) {
	s, dir := nestedFixture(t)
	inner := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first", "tasks", "00-child-one")

	// Remove the inner container's tasks/ directory (with its grandchild):
	// rule 2 flags the missing directory; the dangling grandchild row is a
	// bonus rule-3 violation we do not assert.
	if err := os.RemoveAll(filepath.Join(inner, "tasks")); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	hasViolation(t, s.Validate(), 2, "00-child-one/tasks", "missing tasks/")

	// Remove the inner container's ledger.md: rule 2 flags it; the task
	// file 00-child-one.md keeps its governing row one level up.
	if err := os.Remove(filepath.Join(inner, "ledger.md")); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	hasViolation(t, s.Validate(), 2, "00-child-one/ledger.md", "missing ledger.md")
}

func TestValidateNestedSequenceAndDepth(t *testing.T) {
	s, dir := nestedFixture(t)
	innerLedger := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first", "tasks", "00-child-one", "ledger.md")
	grand := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first", "tasks", "00-child-one", "tasks")

	// Rename the grandchild file and update the row's href to follow it:
	// the sequence now disagrees with the row ID's final segment.
	if err := os.Rename(filepath.Join(grand, "00-grand.md"), filepath.Join(grand, "05-grand.md")); err != nil {
		t.Fatal(err)
	}
	rewrite(t, innerLedger, "(tasks/00-grand.md)", "(tasks/05-grand.md)")
	s.Reload()
	hasViolation(t, s.Validate(), 3, "05-grand.md", "filename sequence")

	// Restore, then make the row ID one level too deep for its nesting.
	if err := os.Rename(filepath.Join(grand, "05-grand.md"), filepath.Join(grand, "00-grand.md")); err != nil {
		t.Fatal(err)
	}
	rewrite(t, innerLedger, "(tasks/05-grand.md)", "(tasks/00-grand.md)")
	rewrite(t, innerLedger, "[FIX-00.00.00]", "[FIX-00.00.00.09]")
	s.Reload()
	hasViolation(t, s.Validate(), 3, "00-grand.md", "depth")

	// A single-digit dotted segment is malformed.
	rewrite(t, innerLedger, "[FIX-00.00.00.09]", "[FIX-00.00.0]")
	s.Reload()
	hasViolation(t, s.Validate(), 3, "00-grand.md", "malformed dotted")
}

func TestValidateOrphanChildFile(t *testing.T) {
	s, dir := nestedFixture(t)
	// Drop the container's ledger row for the second child by rewriting
	// the ledger with only the first child's row.
	containerLedger := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first", "ledger.md")
	tl, err := model.ParseTaskLedger("ledger.md", model.RenderTaskLedger("FIX-00", "2026-09-10-0", "2026-09-16"))
	if err != nil {
		t.Fatal(err)
	}
	tl.AppendTask(model.TaskRow{ID: "FIX-00.00", Href: "tasks/00-child-one.md", Title: "Child One", Status: model.StatusNotStarted, Updated: "2026-09-16", Notes: model.Empty})
	if err := os.WriteFile(containerLedger, tl.Content(), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	hasViolation(t, s.Validate(), 3, "01-child-two.md", "no ledger row")
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
