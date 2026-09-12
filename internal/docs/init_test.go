package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasktracker/internal/docs"
	"tasktracker/internal/store"
)

func actionsByPath(actions []docs.InitAction) map[string]string {
	m := map[string]string{}
	for _, a := range actions {
		m[a.Path] = a.Action
	}
	return m
}

func TestInitEmptyDir(t *testing.T) {
	root := t.TempDir()
	actions, err := docs.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	got := actionsByPath(actions)
	for _, p := range []string{"AGENTS.md", "changes/ledger.md", ".gitignore", "opencode.json", "agentsdocs.json"} {
		if got[p] != "created" {
			t.Errorf("%s: action %q, want created", p, got[p])
		}
	}

	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(agents) != docs.WorkflowInstructions() {
		t.Error("AGENTS.md must be exactly the canonical workflow text")
	}

	// The initialized tree passes workflow validation.
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if v := st.Validate(); len(v) > 0 {
		t.Errorf("initialized repo fails validation: %v", v)
	}

	// Second run is a no-op.
	actions, err = docs.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if a.Action != "skipped" {
			t.Errorf("second run: %s %q, want skipped", a.Path, a.Action)
		}
	}
}

func TestInitExistingAgentsMergedNotClobbered(t *testing.T) {
	root := t.TempDir()
	human := "# My Project\n\nHuman notes about this codebase.\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(human), 0o644); err != nil {
		t.Fatal(err)
	}
	actions, err := docs.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := actionsByPath(actions)["AGENTS.md"]; got != "merged" {
		t.Fatalf("AGENTS.md action %q, want merged", got)
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), human) {
		t.Error("human content not preserved at head of AGENTS.md")
	}
	if !strings.Contains(string(data), strings.TrimRight(docs.WorkflowInstructions(), "\n")) {
		t.Error("workflow text not merged in")
	}

	// Re-init after the workflow text is present must skip, not append again.
	actions, err = docs.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := actionsByPath(actions)["AGENTS.md"]; got != "skipped" {
		t.Errorf("re-init AGENTS.md action %q, want skipped", got)
	}
}

func TestInitGitignoreVariants(t *testing.T) {
	t.Run("existing without entry", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		actions, err := docs.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if got := actionsByPath(actions)[".gitignore"]; got != "merged" {
			t.Fatalf("action %q, want merged", got)
		}
		data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
		if string(data) != "node_modules/\n.tasktracker/\n" {
			t.Errorf("unexpected .gitignore: %q", data)
		}
	})
	t.Run("existing with entry", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".tasktracker/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		actions, err := docs.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if got := actionsByPath(actions)[".gitignore"]; got != "skipped" {
			t.Fatalf("action %q, want skipped", got)
		}
	})
}

func TestInitNeverClobbersExistingConfig(t *testing.T) {
	root := t.TempDir()
	custom := `{"include":["src/**"]}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "agentsdocs.json"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := docs.Init(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "agentsdocs.json"))
	if string(data) != custom {
		t.Error("existing agentsdocs.json was modified")
	}
	// And it still loads.
	if _, err := docs.LoadConfig(root); err != nil {
		t.Fatal(err)
	}
}

// TestWorkflowAssetDrift pins the embedded canonical workflow text to this
// repository's own AGENTS.md: they must stay exactly in sync. The repo's own
// AGENTS.md may grow a machine-maintained auto section (seeded/gardened repo
// docs); the asset is always the human/curated portion, so the comparison
// ignores everything from the begin-marker LINE on (prose mentions of the
// marker string inside the curated text are not a cut point). When AGENTS.md
// changes, refresh the asset with:
//
//	awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md
func TestWorkflowAssetDrift(t *testing.T) {
	rootAgents, err := os.ReadFile(filepath.Join("..", "..", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	human := string(rootAgents)
	cut := -1
	lines := strings.Split(human, "\n")
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "<!-- tasktracker:begin -->" {
			cut = i
			break
		}
	}
	if cut >= 0 {
		human = strings.Join(lines[:cut], "\n")
	}
	if strings.TrimRight(human, "\n") != strings.TrimRight(docs.WorkflowInstructions(), "\n") {
		t.Error("internal/docs/assets/workflow_agents.md differs from repo AGENTS.md; refresh it (see comment above)")
	}
}
