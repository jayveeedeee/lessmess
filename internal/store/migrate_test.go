package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// writeMigrateFixture builds a markdown workflow tree covering the
// migration's surface: legacy numeric and random-suffix change IDs, an
// archived change, a decomposed task with a container ledger, deps,
// notes, empty ("—") cells, and a decision log.
func writeMigrateFixture(t *testing.T) string {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	must(os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("lessmess\n.lessmess/\n"), 0o644))

	root := `# Changes — Root Ledger

| Change | Title | ID prefix | Branch | Status | Created | Last updated |
| --- | --- | --- | --- | --- | --- | --- |
| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | In progress | 2026-09-10 | 2026-09-11 |
| [2026-09-12-abcde](2026-09-12-abcde/plan.md) | Nested change | NTD | main | Done | 2026-09-12 | 2026-09-12 |
| [2026-09-11-0](archive/2026-09-11-0/plan.md) | Archived change | OLD | — | Done | 2026-09-11 | 2026-09-11 |
`
	must(os.MkdirAll(filepath.Join(dir, "changes"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "changes", "ledger.md"), []byte(root), 0o644))

	taskBody := func(id, title string) []byte {
		return []byte("---\nid: " + id + "\ntitle: " + title + "\n---\n\n# " + id + ": " + title + "\n\nBody of " + id + ".\n")
	}

	// Change 1: legacy numeric ID, deps, notes, decision log.
	c1 := filepath.Join(dir, "changes", "2026-09-10-0")
	must(os.MkdirAll(filepath.Join(c1, "tasks"), 0o755))
	must(os.WriteFile(filepath.Join(c1, "plan.md"), model.RenderChangePlan("2026-09-10-0", "Fixture change", "2026-09-10"), 0o644))
	l1 := model.RenderChangeLedger("2026-09-10-0", "2026-09-10")
	cl1, err := model.ParseChangeLedger("l1", l1)
	if err != nil {
		t.Fatal(err)
	}
	cl1.SetOverall(model.OverallInProgress, "2026-09-11")
	cl1.AppendTask(model.TaskRow{ID: "FIX-00", Href: "tasks/00-first.md", Title: "First", Status: model.StatusNotStarted, Updated: "2026-09-10", Notes: model.Empty})
	cl1.AppendTask(model.TaskRow{ID: "FIX-01", Href: "tasks/01-second.md", Title: "Second", Status: model.StatusInProgress, Depends: []string{"FIX-00"}, Updated: "2026-09-11", Notes: "round-trip first"})
	content := string(cl1.Content()) + "\n| Date | Decision |\n| --- | --- |\n| 2026-09-10 | Keep prose in markdown |\n"
	must(os.WriteFile(filepath.Join(c1, "ledger.md"), []byte(content), 0o644))
	must(os.WriteFile(filepath.Join(c1, "tasks", "00-first.md"), taskBody("FIX-00", "First"), 0o644))
	must(os.WriteFile(filepath.Join(c1, "tasks", "01-second.md"), taskBody("FIX-01", "Second"), 0o644))

	// Change 2: random-suffix ID, decomposed task with container.
	c2 := filepath.Join(dir, "changes", "2026-09-12-abcde")
	must(os.MkdirAll(filepath.Join(c2, "tasks", "00-engine", "tasks"), 0o755))
	must(os.WriteFile(filepath.Join(c2, "plan.md"), model.RenderChangePlan("2026-09-12-abcde", "Nested change", "2026-09-12"), 0o644))
	l2 := model.RenderChangeLedger("2026-09-12-abcde", "2026-09-12")
	cl2, err := model.ParseChangeLedger("l2", l2)
	if err != nil {
		t.Fatal(err)
	}
	cl2.SetOverall(model.OverallDone, "2026-09-12")
	cl2.AppendTask(model.TaskRow{ID: "NTD-00", Href: "tasks/00-engine.md", Title: "Engine", Status: model.StatusDone, Updated: "2026-09-12", Notes: model.Empty})
	cl2.AppendTask(model.TaskRow{ID: "NTD-01", Href: "tasks/01-docs.md", Title: "Docs", Status: model.StatusTest, Updated: "2026-09-12", Notes: model.Empty})
	must(os.WriteFile(filepath.Join(c2, "ledger.md"), cl2.Content(), 0o644))
	must(os.WriteFile(filepath.Join(c2, "tasks", "00-engine.md"), taskBody("NTD-00", "Engine"), 0o644))
	must(os.WriteFile(filepath.Join(c2, "tasks", "01-docs.md"), taskBody("NTD-01", "Docs"), 0o644))
	tl := model.RenderTaskLedger("NTD-00", "2026-09-12-abcde", "2026-09-12")
	ctl, err := model.ParseTaskLedger("tl", tl)
	if err != nil {
		t.Fatal(err)
	}
	ctl.AppendTask(model.TaskRow{ID: "NTD-00.00", Href: "tasks/00-urls.md", Title: "URLs", Status: model.StatusDone, Updated: "2026-09-12", Notes: model.Empty})
	must(os.WriteFile(filepath.Join(c2, "tasks", "00-engine", "ledger.md"), ctl.Content(), 0o644))
	must(os.WriteFile(filepath.Join(c2, "tasks", "00-engine", "tasks", "00-urls.md"), taskBody("NTD-00.00", "URLs"), 0o644))

	// Change 3: archived under changes/archive/.
	c3 := filepath.Join(dir, "changes", "archive", "2026-09-11-0")
	must(os.MkdirAll(filepath.Join(c3, "tasks"), 0o755))
	must(os.WriteFile(filepath.Join(c3, "plan.md"), model.RenderChangePlan("2026-09-11-0", "Archived change", "2026-09-11"), 0o644))
	l3 := model.RenderChangeLedger("2026-09-11-0", "2026-09-11")
	cl3, err := model.ParseChangeLedger("l3", l3)
	if err != nil {
		t.Fatal(err)
	}
	cl3.SetOverall(model.OverallDone, "2026-09-11")
	cl3.AppendTask(model.TaskRow{ID: "OLD-00", Href: "tasks/00-old.md", Title: "Old", Status: model.StatusDone, Updated: "2026-09-11", Notes: model.Empty})
	must(os.WriteFile(filepath.Join(c3, "ledger.md"), cl3.Content(), 0o644))
	must(os.WriteFile(filepath.Join(c3, "tasks", "00-old.md"), taskBody("OLD-00", "Old"), 0o644))
	return dir
}

func TestMigrateWorkflow(t *testing.T) {
	dir := writeMigrateFixture(t)
	res, err := MigrateWorkflow(MigrateOptions{Dir: dir})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.Changes != 3 || res.Tasks != 6 || res.Containers != 1 || res.FilesRewritten != 6 || res.LedgersDeleted != 5 {
		t.Errorf("result = %+v", res)
	}

	indexFile, changesDir := workflowPaths(dir)
	idx, err := model.LoadWorkflowIndex(indexFile)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	if len(idx.Changes) != 3 {
		t.Fatalf("index changes = %d", len(idx.Changes))
	}
	if e := idx.Changes[0]; e.ID != "2026-09-10-0" || e.Prefix != "FIX" || e.Branch != "" || e.Archived {
		t.Errorf("entry 0 = %+v", e)
	}
	if e := idx.Changes[1]; e.ID != "2026-09-12-abcde" || e.Branch != "main" {
		t.Errorf("entry 1 = %+v", e)
	}
	if e := idx.Changes[2]; !e.Archived {
		t.Errorf("entry 2 not archived: %+v", e)
	}

	st1, err := model.LoadChangeState(filepath.Join(changesDir, "2026-09-10-0.json"))
	if err != nil {
		t.Fatalf("load state 1: %v", err)
	}
	if st1.Status != (model.ChangeStatus{Value: model.OverallInProgress, Derived: true}) {
		t.Errorf("state 1 status = %+v", st1.Status)
	}
	if len(st1.Decision) != 1 || st1.Decision[0].Decision != "Keep prose in markdown" {
		t.Errorf("decision log = %+v", st1.Decision)
	}
	if ts := st1.Task("FIX-01"); ts == nil || ts.Status != model.StatusInProgress || len(ts.DependsOn) != 1 || ts.DependsOn[0] != "FIX-00" || ts.Notes != "round-trip first" {
		t.Errorf("FIX-01 = %+v", ts)
	}
	if ts := st1.Task("FIX-00"); ts == nil || ts.Notes != "" || ts.File != "tasks/00-first.md" {
		t.Errorf("FIX-00 = %+v", ts)
	}
	if got := st1.Children(""); len(got) != 2 || got[0].ID != "FIX-00" || got[1].ID != "FIX-01" {
		t.Errorf("top-level order = %+v", got)
	}

	st2, err := model.LoadChangeState(filepath.Join(changesDir, "2026-09-12-abcde.json"))
	if err != nil {
		t.Fatalf("load state 2: %v", err)
	}
	if st2.Status != (model.ChangeStatus{Value: model.OverallDone, Derived: false}) {
		t.Errorf("state 2 status = %+v", st2.Status)
	}
	sub := st2.Task("NTD-00.00")
	if sub == nil || sub.Parent != "NTD-00" || sub.File != "tasks/00-engine/tasks/00-urls.md" || sub.Seq != 0 {
		t.Errorf("NTD-00.00 = %+v", sub)
	}
	if got := st2.Children("NTD-00"); len(got) != 1 || got[0].ID != "NTD-00.00" {
		t.Errorf("children = %+v", got)
	}

	// Prose files: frontmatter stripped, body preserved byte-for-byte.
	first, err := os.ReadFile(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != "\n# FIX-00: First\n\nBody of FIX-00.\n" {
		t.Errorf("stripped prose = %q", first)
	}
	// All markdown ledgers are gone.
	for _, p := range []string{
		filepath.Join(dir, "changes", "ledger.md"),
		filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md"),
		filepath.Join(dir, "changes", "2026-09-12-abcde", "ledger.md"),
		filepath.Join(dir, "changes", "2026-09-12-abcde", "tasks", "00-engine", "ledger.md"),
		filepath.Join(dir, "changes", "archive", "2026-09-11-0", "ledger.md"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists (err=%v)", p, err)
		}
	}
	// .gitignore keeps the workflow subtree committable.
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".lessmess/*", "!.lessmess/workflow/"} {
		if !strings.Contains(string(gi), want) {
			t.Errorf(".gitignore missing %q:\n%s", want, gi)
		}
	}
	if strings.Contains(string(gi), ".lessmess/\n") {
		t.Errorf(".gitignore still ignores the whole dir:\n%s", gi)
	}
}

func TestMigrateWorkflowDryRun(t *testing.T) {
	dir := writeMigrateFixture(t)
	res, err := MigrateWorkflow(MigrateOptions{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.Changes != 3 || res.FilesRewritten != 0 || res.LedgersDeleted != 0 {
		t.Errorf("dry-run result = %+v", res)
	}
	indexFile, _ := workflowPaths(dir)
	if _, err := os.Stat(indexFile); !os.IsNotExist(err) {
		t.Error("dry run wrote the index")
	}
	if _, err := os.Stat(filepath.Join(dir, "changes", "ledger.md")); err != nil {
		t.Error("dry run removed the root ledger")
	}
	first, err := os.ReadFile(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first.md"))
	if err != nil || !strings.HasPrefix(string(first), "---\n") {
		t.Error("dry run rewrote task prose")
	}
}

func TestMigrateWorkflowRefusesWhenJSONExists(t *testing.T) {
	dir := writeMigrateFixture(t)
	indexFile, _ := workflowPaths(dir)
	if err := os.MkdirAll(filepath.Dir(indexFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateWorkflow(MigrateOptions{Dir: dir}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v, want already-exists refusal", err)
	}
}

func TestMigrateWorkflowInvalidTrees(t *testing.T) {
	cases := []struct {
		name  string
		mutst func(dir string)
		want  string
	}{
		{"task file without frontmatter", func(dir string) {
			p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first.md")
			os.WriteFile(p, []byte("# no frontmatter\n"), 0o644)
		}, "frontmatter"},
		{"ledger row without file", func(dir string) {
			p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first.md")
			os.Remove(p)
		}, "no matching task file"},
		{"stray directory", func(dir string) {
			os.MkdirAll(filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "07-stray"), 0o755)
		}, "stray directory"},
		{"frontmatter/row id mismatch", func(dir string) {
			p := filepath.Join(dir, "changes", "2026-09-10-0", "tasks", "00-first.md")
			os.WriteFile(p, []byte("---\nid: FIX-99\ntitle: First\n---\n\nbody\n"), 0o644)
		}, "does not match ledger row"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeMigrateFixture(t)
			tc.mutst(dir)
			_, err := MigrateWorkflow(MigrateOptions{Dir: dir})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want substring %q", err, tc.want)
			}
			indexFile, _ := workflowPaths(dir)
			if _, err := os.Stat(indexFile); !os.IsNotExist(err) {
				t.Error("failed migration still wrote state")
			}
			if _, err := os.Stat(filepath.Join(dir, "changes", "ledger.md")); err != nil {
				t.Error("failed migration removed the root ledger")
			}
		})
	}
}

func TestMigrateWorkflowWorktreeChange(t *testing.T) {
	dir := writeMigrateFixture(t)
	// Move change 1 out of the main tree, as a worktree-backed change.
	wt := t.TempDir()
	c1 := filepath.Join(dir, "changes", "2026-09-10-0")
	if err := os.Rename(c1, filepath.Join(wt, "2026-09-10-0")); err != nil {
		t.Fatal(err)
	}
	res, err := MigrateWorkflow(MigrateOptions{
		Dir: dir,
		ChangeRoot: func(id string) (string, bool) {
			if id == "2026-09-10-0" {
				return filepath.Join(wt, id), true
			}
			return "", false
		},
	})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.Changes != 3 {
		t.Errorf("changes = %d", res.Changes)
	}
	if _, err := os.Stat(filepath.Join(wt, "2026-09-10-0", "ledger.md")); !os.IsNotExist(err) {
		t.Error("worktree change ledger not deleted")
	}
	first, err := os.ReadFile(filepath.Join(wt, "2026-09-10-0", "tasks", "00-first.md"))
	if err != nil || strings.HasPrefix(string(first), "---") {
		t.Error("worktree prose not rewritten")
	}
}

func TestPatchGitignoreVariants(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"directory ignore", "node_modules/\n.lessmess/\n", "node_modules/\n.lessmess/*\n!.lessmess/workflow/\n"},
		{"legacy state dir", ".tasktracker/\n", ".lessmess/*\n!.lessmess/workflow/\n"},
		{"already patched", ".lessmess/*\n!.lessmess/workflow/\n", ".lessmess/*\n!.lessmess/workflow/\n"},
		{"no mention", "node_modules/\n", "node_modules/\n\n.lessmess/*\n!.lessmess/workflow/\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(tc.in), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := patchGitignore(dir); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("gitignore = %q, want %q", got, tc.want)
			}
			// Idempotent.
			if err := patchGitignore(dir); err != nil {
				t.Fatal(err)
			}
			got2, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
			if string(got2) != tc.want {
				t.Errorf("second run = %q, want unchanged %q", got2, tc.want)
			}
		})
	}
}
