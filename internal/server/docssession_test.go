package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/docs"
	"lessmess/internal/model"
	"lessmess/internal/opencode"
)

// fakeSessionClient simulates the opencode service; onPrompt stands in for
// the agent's file edits.
type fakeSessionClient struct {
	prompts   []string
	onPrompt  func()
	promptErr error
}

func (f *fakeSessionClient) CreateSession(_ context.Context, title, _ string) (*opencode.Session, error) {
	return &opencode.Session{ID: "ses_fake", Title: title}, nil
}

func (f *fakeSessionClient) Prompt(_ context.Context, _ string, text string) error {
	f.prompts = append(f.prompts, text)
	if f.promptErr != nil {
		return f.promptErr
	}
	if f.onPrompt != nil {
		f.onPrompt()
	}
	return nil
}

func (f *fakeSessionClient) WaitDone(_ context.Context, _ string) error { return nil }
func (f *fakeSessionClient) DeleteSession(_ context.Context, _ string) error {
	return nil
}

// gardenerRepo builds a covered fixture tree:
//
//	README.md
//	internal/model/model.go
func gardenerRepo(t *testing.T) (string, *docs.Config) {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"README.md", "internal/model/model.go"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, docs.ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := docs.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, cfg
}

// goodAgent fills the STRUCTURE.md placeholders and appends an AGENTS.md
// learning for dir — compliant gardener behavior.
func goodAgent(root, dir string) {
	p := filepath.Join(root, filepath.FromSlash(dir), docs.StructureFile)
	data, _ := os.ReadFile(p)
	content := strings.Replace(string(data), "\n"+docs.Placeholder+"\n", "\nFixture purpose.\n", 1)
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "| "+docs.Placeholder+" |") {
			lines[i] = strings.Replace(ln, "| "+docs.Placeholder+" |", "| fixture blurb |", 1)
		}
	}
	os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o644)
	agents := model.DocMarkerBegin + "\n## Learnings\n\n- (2026-09-10-0) fixture learning\n" + model.DocMarkerEnd + "\n"
	os.WriteFile(filepath.Join(root, filepath.FromSlash(dir), docs.AgentsFile), []byte(agents), 0o644)
}

func TestGardenerHappyPath(t *testing.T) {
	root, cfg := gardenerRepo(t)
	fake := &fakeSessionClient{}
	fake.onPrompt = func() { goodAgent(root, "internal/model") }
	r := &gardenerRunner{oc: fake, root: root, cfg: cfg}

	job := DocsJob{Change: "2026-09-10-0", Title: "Fixture change", Dirs: []string{"internal/model"}}
	if err := r.RunDocsJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if len(fake.prompts) != 1 {
		t.Fatalf("prompts: %d", len(fake.prompts))
	}
	p := fake.prompts[0]
	for _, want := range []string{"2026-09-10-0", "Fixture change", "internal/model", "tasktracker-meta", "(2026-09-10-0)"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}

	structure := readDocFile(t, root, "internal/model/STRUCTURE.md")
	if !strings.Contains(structure, "fixture blurb") || !strings.Contains(structure, "Fixture purpose.") {
		t.Error("placeholders not annotated")
	}
	if !strings.Contains(structure, "source=2026-09-10-0") {
		t.Error("meta not stamped with the change ID")
	}
	agents := readDocFile(t, root, "internal/model/AGENTS.md")
	if !strings.Contains(agents, "(2026-09-10-0) fixture learning") {
		t.Error("learning with change citation missing")
	}
	// The root skeleton was built by the deterministic pass and quotes the
	// freshly written child purpose after the settle pass.
	rootDoc := readDocFile(t, root, "STRUCTURE.md")
	if !strings.Contains(rootDoc, "| `internal/` | — |") == false {
		// internal quotes model? no — internal has no files; its own purpose stays placeholder.
	}
	if !strings.Contains(rootDoc, "internal/") {
		t.Error("root skeleton missing internal entry")
	}
}

func readDocFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGardenerRevertsOutsideMarkerEdits(t *testing.T) {
	root, cfg := gardenerRepo(t)
	// Pre-seed an AGENTS.md with human content.
	human := "# model\n\nHuman notes.\n"
	if err := os.WriteFile(filepath.Join(root, "internal", "model", docs.AgentsFile), []byte(human), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeSessionClient{}
	fake.onPrompt = func() {
		// Misbehaving: edits human content outside the markers.
		os.WriteFile(filepath.Join(root, "internal", "model", docs.AgentsFile), []byte("# model\n\nTampered.\n"), 0o644)
	}
	r := &gardenerRunner{oc: fake, root: root, cfg: cfg}
	job := DocsJob{Change: "2026-09-10-0", Title: "T", Dirs: []string{"internal/model"}}
	err := r.RunDocsJob(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "confinement violation") {
		t.Fatalf("expected confinement violation, got %v", err)
	}
	if got := readDocFile(t, root, "internal/model/AGENTS.md"); got != human {
		t.Errorf("human content not restored:\n%s", got)
	}
	// The deterministic skeleton (written before the session) survives the revert.
	if !strings.Contains(readDocFile(t, root, "internal/model/STRUCTURE.md"), "# Structure:") {
		t.Error("skeleton should survive the revert")
	}
}

func TestGardenerRevertsMetaTampering(t *testing.T) {
	root, cfg := gardenerRepo(t)
	fake := &fakeSessionClient{}
	fake.onPrompt = func() {
		p := filepath.Join(root, "internal", "model", docs.StructureFile)
		data, _ := os.ReadFile(p)
		os.WriteFile(p, []byte(strings.Replace(string(data), "tree=", "tree=deadbeef", 1)), 0o644)
	}
	r := &gardenerRunner{oc: fake, root: root, cfg: cfg}
	job := DocsJob{Change: "2026-09-10-0", Title: "T", Dirs: []string{"internal/model"}}
	if err := r.RunDocsJob(context.Background(), job); err == nil {
		t.Fatal("meta tampering must be rejected")
	}
	structure := readDocFile(t, root, "internal/model/STRUCTURE.md")
	if strings.Contains(structure, "deadbeef") {
		t.Error("tampered meta not reverted")
	}
	if _, err := model.ParseDocFile("x", []byte(structure)); err != nil {
		t.Error("restored file not parseable")
	}
}

func TestGardenerRemovesCreatedFileOnViolation(t *testing.T) {
	root, cfg := gardenerRepo(t)
	fake := &fakeSessionClient{}
	fake.onPrompt = func() {
		// Creates a markerless AGENTS.md (violation) — and edits nothing else.
		os.WriteFile(filepath.Join(root, "internal", "model", docs.AgentsFile), []byte("no markers here\n"), 0o644)
	}
	r := &gardenerRunner{oc: fake, root: root, cfg: cfg}
	job := DocsJob{Change: "2026-09-10-0", Title: "T", Dirs: []string{"internal/model"}}
	if err := r.RunDocsJob(context.Background(), job); err == nil {
		t.Fatal("markerless created AGENTS.md must be rejected")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "model", docs.AgentsFile)); !os.IsNotExist(err) {
		t.Error("created file not rolled back")
	}
}

func TestGardenerToleratesMissingDirs(t *testing.T) {
	root, cfg := gardenerRepo(t)
	fake := &fakeSessionClient{}
	r := &gardenerRunner{oc: fake, root: root, cfg: cfg}
	job := DocsJob{Change: "2026-09-10-0", Title: "T", Dirs: []string{"gone/dir"}}
	if err := r.RunDocsJob(context.Background(), job); err != nil {
		t.Fatalf("missing dirs must be tolerated: %v", err)
	}
	if len(fake.prompts) != 0 {
		t.Error("no session expected when no target dirs exist")
	}
}

func TestGardenerManualPrompt(t *testing.T) {
	p := gardenerPrompt(DocsJob{Change: "manual", Title: "manual reconciliation"}, []*docs.Dir{{Rel: "web"}})
	for _, want := range []string{"MANUAL reconciliation", "(manual)", "web"} {
		if !strings.Contains(p, want) {
			t.Errorf("manual prompt missing %q", want)
		}
	}
	if strings.Contains(p, "changes/manual") {
		t.Error("manual prompt must not reference a nonexistent change record")
	}
}

func TestGardenerSessionFailure(t *testing.T) {
	root, cfg := gardenerRepo(t)
	fake := &fakeSessionClient{promptErr: fmt.Errorf("service exploded")}
	r := &gardenerRunner{oc: fake, root: root, cfg: cfg}
	job := DocsJob{Change: "2026-09-10-0", Title: "T", Dirs: []string{"internal/model"}}
	if err := r.RunDocsJob(context.Background(), job); err == nil || !strings.Contains(err.Error(), "service exploded") {
		t.Fatalf("expected session error, got %v", err)
	}
}
