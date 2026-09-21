package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// worktreeChangeID is the id used by the worktree-resolution fixtures.
const worktreeChangeID = "2026-09-10-aaaaa"

// writeWorktreeFixture registers a worktree-backed change: an index entry
// and a central state file, but NO prose directory in the main tree.
func writeWorktreeFixture(t *testing.T, dir string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wd := filepath.Join(dir, StateDirName, "workflow")
	idx, err := model.LoadWorkflowIndex(filepath.Join(wd, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	idx.Changes = append(idx.Changes, model.IndexEntry{
		ID: worktreeChangeID, Title: "Worktree change", Prefix: "WTC",
		Branch: "change/2026-09-10-aaaaa", Created: "2026-09-10",
	})
	must(idx.Save(filepath.Join(wd, "index.json")))
	st := &model.ChangeState{
		Version: model.StateVersion,
		ID:      worktreeChangeID,
		Title:   "Worktree change",
		Prefix:  "WTC",
		Branch:  "change/2026-09-10-aaaaa",
		Status:  model.ChangeStatus{Value: model.OverallPlanned, Derived: true},
		Created: "2026-09-10",
		Updated: "2026-09-10",
		Tasks: []model.TaskState{
			{ID: "WTC-00", Seq: 0, Title: "Only", File: "tasks/00-only.md", Status: model.StatusNotStarted, Updated: "2026-09-10"},
		},
	}
	must(st.Save(filepath.Join(wd, "changes", worktreeChangeID+".json")))
}

// buildWorktreeChange writes the change's prose directory into the
// "worktree" (outside the main tree).
func buildWorktreeChange(t *testing.T, wt, id string) string {
	t.Helper()
	cdir := filepath.Join(wt, "changes", id)
	if err := os.MkdirAll(filepath.Join(cdir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan(id, "Worktree change", "2026-09-10"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "tasks", "00-only.md"), model.RenderTaskFile("WTC-00", "Only"), 0o644); err != nil {
		t.Fatal(err)
	}
	return cdir
}

func TestWorktreeResolver(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	writeWorktreeFixture(t, dir)
	wt := t.TempDir()
	cdir := buildWorktreeChange(t, wt, worktreeChangeID)

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	// Without a resolver the entry is an unresolvable change.
	if _, err := s.Change(worktreeChangeID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Change = %v, want ErrNotFound", err)
	}
	var rule6 bool
	for _, v := range s.Validate() {
		if v.Rule == 6 && strings.Contains(v.Msg, worktreeChangeID) {
			rule6 = true
		}
	}
	if !rule6 {
		t.Error("expected a rule-6 violation for the unresolvable entry")
	}

	// Wire the resolver: the change resolves from the worktree.
	s.SetChangeRoot(func(id string) (string, bool) {
		if id != worktreeChangeID {
			return "", false
		}
		return cdir, true
	})

	c, err := s.Change(worktreeChangeID)
	if err != nil {
		t.Fatalf("Change after resolver: %v", err)
	}
	if c.Dir != cdir {
		t.Errorf("Change.Dir = %q, want %q", c.Dir, cdir)
	}
	if len(c.Roots) != 1 || c.Roots[0].ID != "WTC-00" {
		t.Fatalf("worktree tree = %+v", c.Roots)
	}
	plan, err := s.PlanFile(worktreeChangeID)
	if err != nil || !strings.Contains(plan, "Worktree change") {
		t.Errorf("PlanFile = %q, %v", plan, err)
	}
	tf, err := s.TaskFile(worktreeChangeID, "tasks/00-only.md")
	if err != nil || tf.ID != "WTC-00" {
		t.Errorf("TaskFile = %+v, %v", tf, err)
	}
	for _, v := range s.Validate() {
		if strings.Contains(v.Msg, worktreeChangeID) {
			t.Errorf("unexpected violation after resolution: %v", v)
		}
	}

	// Store writes go to the central state, never into the worktree.
	if err := s.MoveTask(worktreeChangeID, "WTC-00", model.StatusInProgress, 0); err != nil {
		t.Fatalf("MoveTask worktree change: %v", err)
	}
	st := loadState(t, dir, worktreeChangeID)
	if st.Task("WTC-00").Status != model.StatusInProgress {
		t.Fatalf("worktree move did not land in central state: %+v", st.Tasks)
	}
}
