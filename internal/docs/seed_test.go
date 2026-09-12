package docs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// stubSummarizer simulates an agent pass: it fills placeholders in
// STRUCTURE.md and writes an AGENTS.md auto section — unless the dir is in
// bad, in which case it corrupts STRUCTURE.md.
type stubSummarizer struct {
	calls []string
	bad   map[string]bool
}

func (s *stubSummarizer) SummarizeDir(_ context.Context, _ string, d *Dir) error {
	s.calls = append(s.calls, d.Rel)
	p := filepath.Join(d.Abs, StructureFile)
	if s.bad[d.Rel] {
		return os.WriteFile(p, []byte("garbage\n"+model.DocMarkerEnd+"\n"), 0o644)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	content := strings.Replace(string(data), "\n"+Placeholder+"\n", "\nStub purpose for "+d.Rel+".\n", 1)
	// Compliant-agent behavior: only file-entry rows get blurbs; covered
	// subdir rows quote the child's purpose and are left alone.
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "/` |") {
			continue
		}
		if strings.Contains(ln, "| "+Placeholder+" |") {
			lines[i] = strings.Replace(ln, "| "+Placeholder+" |", "| stub blurb |", 1)
		}
	}
	content = strings.Join(lines, "\n")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return err
	}
	agents := model.DocMarkerBegin + "\n## Learnings\n\n- stub learning for " + d.Rel + "\n" + model.DocMarkerEnd + "\n"
	return os.WriteFile(filepath.Join(d.Abs, AgentsFile), []byte(agents), 0o644)
}

// mkSeedTree builds a fixture repo without pre-existing doc files:
//
//	README.md
//	cmd/main.go
//	internal/model/model.go
//	node_modules/pkg/index.js   (uncovered)
//	agentsdocs.json             (docs enabled, default coverage)
func mkSeedTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"README.md", "cmd/main.go", "internal/model/model.go", "node_modules/pkg/index.js"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSeedHappyPath(t *testing.T) {
	root := mkSeedTree(t)
	sum := &stubSummarizer{}
	var out strings.Builder
	err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{Date: "2026-09-12"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	// Post-order: cmd, internal/model, internal, .
	wantCalls := []string{"cmd", "internal/model", "internal", "."}
	if strings.Join(sum.calls, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("calls %v, want %v", sum.calls, wantCalls)
	}
	leaf := readFile(t, root, "internal/model/STRUCTURE.md")
	if !strings.Contains(leaf, "Stub purpose for internal/model.") {
		t.Error("purpose not filled")
	}
	if strings.Contains(leaf, "| "+Placeholder+" |") {
		t.Error("placeholders remain")
	}
	if !strings.Contains(leaf, "source=seed") {
		t.Error("meta missing")
	}
	agents := readFile(t, root, "internal/model/AGENTS.md")
	if !strings.Contains(agents, "stub learning") || !strings.Contains(agents, model.DocMarkerBegin) {
		t.Error("AGENTS.md learnings not written")
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules", StructureFile)); !os.IsNotExist(err) {
		t.Error("uncovered dir must not get docs")
	}
}

func TestSeedIdempotent(t *testing.T) {
	root := mkSeedTree(t)
	sum := &stubSummarizer{}
	opts := SeedOptions{Date: "2026-09-12"}
	var out strings.Builder
	if err := Seed(context.Background(), root, DefaultConfig(), sum, opts, &out); err != nil {
		t.Fatal(err)
	}
	first := map[string]string{}
	for _, rel := range []string{"STRUCTURE.md", "AGENTS.md", "cmd/STRUCTURE.md", "internal/STRUCTURE.md", "internal/model/STRUCTURE.md"} {
		first[rel] = readFile(t, root, rel)
	}
	calls := len(sum.calls)

	out.Reset()
	if err := Seed(context.Background(), root, DefaultConfig(), sum, opts, &out); err != nil {
		t.Fatal(err)
	}
	if len(sum.calls) != calls {
		t.Errorf("second run called summarizer for done dirs: %v", sum.calls[calls:])
	}
	for rel, want := range first {
		if got := readFile(t, root, rel); got != want {
			t.Errorf("%s changed on rerun:\n--- first ---\n%s\n--- second ---\n%s", rel, want, got)
		}
	}
	if strings.Contains(out.String(), "(summarized)") {
		t.Errorf("second run must not summarize:\n%s", out.String())
	}
}

func TestSeedBudgetAndResume(t *testing.T) {
	root := mkSeedTree(t)
	sum := &stubSummarizer{}
	var out strings.Builder
	if err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{Date: "2026-09-12", Budget: 1}, &out); err != nil {
		t.Fatal(err)
	}
	if len(sum.calls) != 1 {
		t.Fatalf("budget 1: calls %v", sum.calls)
	}
	if !strings.Contains(out.String(), "budget exhausted") {
		t.Error("remaining dirs must report budget exhaustion")
	}
	out.Reset()
	if err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{Date: "2026-09-12"}, &out); err != nil {
		t.Fatal(err)
	}
	if len(sum.calls) != 4 {
		t.Errorf("resume: calls %v, want 4 total", sum.calls)
	}
	leaf := readFile(t, root, "internal/model/STRUCTURE.md")
	if strings.Contains(leaf, "| "+Placeholder+" |") {
		t.Error("placeholders remain after resume")
	}
}

func TestSeedDryRun(t *testing.T) {
	root := mkSeedTree(t)
	sum := &stubSummarizer{}
	var out strings.Builder
	if err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{DryRun: true, Date: "2026-09-12"}, &out); err != nil {
		t.Fatal(err)
	}
	if len(sum.calls) != 0 {
		t.Error("dry run must not call the summarizer")
	}
	if !strings.Contains(out.String(), "dry-run:new") || !strings.Contains(out.String(), "would summarize") {
		t.Errorf("dry-run output:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, StructureFile)); !os.IsNotExist(err) {
		t.Error("dry run wrote STRUCTURE.md")
	}
	if _, err := os.Stat(filepath.Join(root, seedCursorPath)); !os.IsNotExist(err) {
		t.Error("dry run wrote cursor")
	}
}

func TestSeedBadAgentRestored(t *testing.T) {
	root := mkSeedTree(t)
	sum := &stubSummarizer{bad: map[string]bool{"internal/model": true}}
	var out strings.Builder
	err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{Date: "2026-09-12"}, &out)
	if err == nil || !strings.Contains(err.Error(), "internal/model") {
		t.Fatalf("expected failure for internal/model, got %v", err)
	}
	// STRUCTURE.md rolled back to the skeleton (placeholders intact, parseable).
	leaf := readFile(t, root, "internal/model/STRUCTURE.md")
	if !strings.Contains(leaf, "| "+Placeholder+" |") {
		t.Error("corrupted file not rolled back to skeleton")
	}
	if _, err := model.ParseDocFile("STRUCTURE.md", []byte(leaf)); err != nil {
		t.Error("rolled-back file not parseable")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "model", AgentsFile)); !os.IsNotExist(err) {
		t.Error("AGENTS.md created by bad pass must be rolled back")
	}
	// Other dirs still processed; failed dir not in cursor.
	for _, rel := range []string{"cmd", "internal", "."} {
		if !strings.Contains(readFile(t, root, filepath.Join(rel, "STRUCTURE.md")), "Stub purpose") {
			t.Errorf("%s not summarized despite sibling failure", rel)
		}
	}
	cursor := loadSeedCursor(root)
	if cursor.Summarized["internal/model"] {
		t.Error("failed dir must not be marked summarized")
	}
	if !cursor.Summarized["cmd"] || !cursor.Summarized["internal"] || !cursor.Summarized["."] {
		t.Error("successful dirs missing from cursor")
	}
}

func TestSeedOffline(t *testing.T) {
	root := mkSeedTree(t)
	var out strings.Builder
	if err := Seed(context.Background(), root, DefaultConfig(), nil, SeedOptions{Date: "2026-09-12"}, &out); err != nil {
		t.Fatal(err)
	}
	leaf := readFile(t, root, "internal/model/STRUCTURE.md")
	if !strings.Contains(leaf, "| "+Placeholder+" |") {
		t.Error("offline skeleton must keep placeholders")
	}
	if !strings.Contains(out.String(), "pending: no summarizer") {
		t.Errorf("output:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, seedCursorPath)); !os.IsNotExist(err) {
		t.Error("offline run must not write cursor")
	}
}

func TestSeedPrompt(t *testing.T) {
	d := &Dir{Rel: "internal/model", Files: []string{"model.go"}, Subdirs: []string{"sub"}}
	p := SeedPrompt(d)
	for _, want := range []string{"internal/model", "model.go", "sub/", "tasktracker-meta", "AGENTS.md", "STRUCTURE.md"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
