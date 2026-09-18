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

// writeWorktreeFixture adds a root-ledger row for worktreeChangeID to the
// standard fixture without creating its directory — the worktree state.
func writeWorktreeFixture(t *testing.T, dir, root string) {
	t.Helper()
	path := filepath.Join(dir, "changes", "ledger.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	extended := strings.Replace(string(data),
		"| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | In progress | 2026-09-10 | 2026-09-10 |",
		"| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | In progress | 2026-09-10 | 2026-09-10 |\n"+
			"| [2026-09-10-aaaaa](2026-09-10-aaaaa/plan.md) | Worktree change | WTC | change/2026-09-10-aaaaa | Planned | 2026-09-10 | 2026-09-10 |",
		1)
	if extended == string(data) {
		t.Fatal("root fixture row not found")
	}
	if err := os.WriteFile(path, []byte(extended), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = root
}

// buildWorktreeChange writes a valid change directory into the "worktree".
func buildWorktreeChange(t *testing.T, wt, id string) string {
	t.Helper()
	cdir := filepath.Join(wt, "changes", id)
	if err := os.MkdirAll(filepath.Join(cdir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan(id, "Worktree change", "2026-09-10"), 0o644))
	ledger := model.RenderChangeLedger(id, "2026-09-10")
	l, err := model.ParseChangeLedger("ledger.md", ledger)
	if err != nil {
		t.Fatal(err)
	}
	l.AppendTask(model.TaskRow{ID: "WTC-00", Href: "tasks/00-only.md", Title: "Only", Status: model.StatusNotStarted, Updated: "2026-09-10", Notes: model.Empty})
	must(os.WriteFile(filepath.Join(cdir, "ledger.md"), l.Content(), 0o644))
	must(os.WriteFile(filepath.Join(cdir, "tasks", "00-only.md"), model.RenderTaskFile("WTC-00", "Only"), 0o644))
	return cdir
}

func TestWorktreeResolver(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	writeWorktreeFixture(t, dir, "")
	wt := t.TempDir()
	cdir := buildWorktreeChange(t, wt, worktreeChangeID)

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	// Without a resolver the row is an unresolvable change.
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
		t.Error("expected a rule-6 violation for the unresolvable row")
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
}

func TestWorktreeResolverStale(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	writeWorktreeFixture(t, dir, "")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	// A resolver that names a vanished directory self-heals to unresolved.
	s.SetChangeRoot(func(id string) (string, bool) {
		if id != worktreeChangeID {
			return "", false
		}
		return filepath.Join(t.TempDir(), "gone", id), true
	})
	if _, err := s.Change(worktreeChangeID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Change = %v, want ErrNotFound for a stale worktree", err)
	}
}

func TestWorktreeCrossTreeStatus(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	writeWorktreeFixture(t, dir, "")
	wt := t.TempDir()
	cdir := buildWorktreeChange(t, wt, worktreeChangeID)

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.SetChangeRoot(func(id string) (string, bool) {
		if id != worktreeChangeID {
			return "", false
		}
		return cdir, true
	})

	// Task work lands in the worktree ledger.
	if err := s.MoveTask(worktreeChangeID, "WTC-00", model.StatusTest, 0); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	ready, offending, err := s.CloseOutReady(worktreeChangeID)
	if err != nil || !ready {
		t.Fatalf("CloseOutReady = %v, %v, %v", ready, offending, err)
	}

	// Overall status writes cross-tree: worktree ledger + main-tree row.
	if err := s.SetChangeStatus(worktreeChangeID, model.OverallDone); err != nil {
		t.Fatalf("SetChangeStatus: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(cdir, "ledger.md"))
	if err != nil || !strings.Contains(string(data), "Done") {
		t.Errorf("worktree ledger after close = %q, %v", data, err)
	}
	rootData, err := os.ReadFile(filepath.Join(dir, "changes", "ledger.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rootData), "| Done |") {
		t.Errorf("main-tree root ledger missing Done row:\n%s", rootData)
	}
	if _, err := os.Stat(filepath.Join(dir, "changes", worktreeChangeID)); !os.IsNotExist(err) {
		t.Error("worktree change must not materialize in the main tree")
	}
}

func TestMintChangeIDAvoidsRootRows(t *testing.T) {
	s, _ := openFixture(t)
	// A worktree change of the same date exists as a row but not a dir;
	// minting must skip it.
	if err := os.WriteFile(filepath.Join(s.ChangesDir, "ledger.md"), []byte(`# Changes — Root Ledger

| Change | Title | ID prefix | Branch | Status | Created | Last updated |
| --- | --- | --- | --- | --- | --- | --- |
| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | In progress | 2026-09-10 | 2026-09-10 |
| [2026-09-10-aaaaa](2026-09-10-aaaaa/plan.md) | Worktree change | WTC | — | Planned | 2026-09-10 | 2026-09-10 |
`), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()

	orig := randSuffix
	defer func() { randSuffix = orig }()
	sequence := []string{"aaaaa", "bbbbb"}
	randSuffix = func() string {
		next := sequence[0]
		sequence = sequence[1:]
		return next
	}
	id, err := s.MintChangeID("2026-09-10")
	if err != nil {
		t.Fatal(err)
	}
	if id != "2026-09-10-bbbbb" {
		t.Errorf("MintChangeID = %q, want 2026-09-10-bbbbb (aaaaa is taken by a row)", id)
	}
}

func TestCreateChangeAt(t *testing.T) {
	s, dir := openFixture(t)
	wt := t.TempDir()
	id := "2026-09-11-ccccc"
	if err := s.CreateChangeAt(id, wt, "Worktree change", "WTC", "change/"+id, "2026-09-11"); err != nil {
		t.Fatalf("CreateChangeAt: %v", err)
	}
	for _, req := range []string{"plan.md", "ledger.md", "tasks"} {
		if _, err := os.Stat(filepath.Join(wt, id, req)); err != nil {
			t.Errorf("%s missing under the worktree root: %v", req, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "changes", id)); !os.IsNotExist(err) {
		t.Error("CreateChangeAt must not write the change dir into the main tree")
	}
	rootData, err := os.ReadFile(filepath.Join(dir, "changes", "ledger.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rootData), id) || !strings.Contains(string(rootData), "change/"+id) {
		t.Errorf("root ledger row missing or branch not recorded:\n%s", rootData)
	}
	// Bad IDs are rejected before any write.
	if err := s.CreateChangeAt("not-an-id", wt, "T", "", "", "2026-09-11"); !errors.Is(err, ErrInvalid) {
		t.Errorf("malformed id = %v, want ErrInvalid", err)
	}
}
