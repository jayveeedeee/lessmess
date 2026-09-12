package docs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findingsByFile(fs []Finding) map[string]Finding {
	m := map[string]Finding{}
	for _, f := range fs {
		if _, ok := m[f.File]; !ok {
			m[f.File] = f
		}
	}
	return m
}

func TestValidateDocsDisabled(t *testing.T) {
	if got := ValidateDocs(t.TempDir(), nil); got != nil {
		t.Errorf("disabled repo must yield no findings, got %v", got)
	}
}

func TestValidateDocsUnseeded(t *testing.T) {
	root := mkSeedTree(t)
	got := ValidateDocs(root, nil)
	m := findingsByFile(got)
	// Every covered dir reports both missing docs as warnings, nothing as error.
	for _, f := range got {
		if f.Severity != SeverityWarning {
			t.Errorf("unseeded repo must warn, not error: %v", f)
		}
	}
	for _, want := range []string{"STRUCTURE.md", "AGENTS.md", "cmd/STRUCTURE.md", "internal/model/STRUCTURE.md"} {
		if _, ok := m[want]; !ok {
			t.Errorf("missing finding for %s in %v", want, m)
		}
	}
}

func TestValidateDocsHealthyAfterSeed(t *testing.T) {
	root := mkSeedTree(t)
	sum := &stubSummarizer{}
	var out strings.Builder
	if err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{Date: "2026-09-12"}, &out); err != nil {
		t.Fatal(err)
	}
	if got := ValidateDocs(root, nil); len(got) != 0 {
		t.Errorf("seeded repo must be healthy, got %v", got)
	}
}

func TestValidateDocsStaleHash(t *testing.T) {
	root := mkSeedTree(t)
	sum := &stubSummarizer{}
	var out strings.Builder
	if err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{Date: "2026-09-12"}, &out); err != nil {
		t.Fatal(err)
	}
	// Add a file: the tree changes under the existing docs.
	if err := os.WriteFile(filepath.Join(root, "internal", "model", "new.go"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ValidateDocs(root, nil)
	m := findingsByFile(got)
	f, ok := m["internal/model/STRUCTURE.md"]
	if !ok || f.Severity != SeverityWarning || !strings.Contains(f.Msg, "stale") {
		t.Errorf("expected stale warning for internal/model, got %v", got)
	}
	// The staleness bubbles to ancestors (root hash changed too).
	if _, ok := m["STRUCTURE.md"]; !ok {
		t.Errorf("root STRUCTURE.md must also be stale, got %v", got)
	}
}

func TestValidateDocsCorruptMarkers(t *testing.T) {
	root := mkSeedTree(t)
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", StructureFile), []byte("x\n<!-- tasktracker:end -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ValidateDocs(root, nil)
	m := findingsByFile(got)
	f, ok := m["cmd/STRUCTURE.md"]
	if !ok || f.Severity != SeverityError {
		t.Errorf("corrupt markers must be an error finding, got %v", got)
	}
}

func TestValidateDocsReportsQueueStale(t *testing.T) {
	root := mkSeedTree(t)
	stale := map[string]string{"internal/model": "opencode service unavailable"}
	got := ValidateDocs(root, stale)
	found := false
	for _, f := range got {
		if f.File == "internal/model/STRUCTURE.md" && strings.Contains(f.Msg, "docs queue") {
			found = true
		}
	}
	if !found {
		t.Errorf("queue stale flag not reported: %v", got)
	}
}

func TestStaleDirs(t *testing.T) {
	// Disabled repo: nil.
	if got, err := StaleDirs(t.TempDir()); err != nil || got != nil {
		t.Errorf("disabled: %v, %v", got, err)
	}

	root := mkSeedTree(t)
	sum := &stubSummarizer{}
	var out strings.Builder
	if err := Seed(context.Background(), root, DefaultConfig(), sum, SeedOptions{Date: "2026-09-12"}, &out); err != nil {
		t.Fatal(err)
	}
	// Fresh after seed.
	got, err := StaleDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("fresh after seed: %v", got)
	}

	// Add a file: dir + ancestors go stale.
	if err := os.WriteFile(filepath.Join(root, "internal", "model", "new.go"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = StaleDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".", "internal", "internal/model"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("after file add: got %v, want %v", got, want)
	}

	// Missing STRUCTURE.md counts as stale.
	if err := os.Remove(filepath.Join(root, "cmd", StructureFile)); err != nil {
		t.Fatal(err)
	}
	got, err = StaleDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range got {
		if d == "cmd" {
			found = true
		}
	}
	if !found {
		t.Errorf("deleted STRUCTURE.md not reported stale: %v", got)
	}
}
