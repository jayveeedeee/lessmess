package docs

import (
	"os"
	"path/filepath"
	"testing"
)

// mkTree builds a fixture repo:
//
//	README.md
//	cmd/main.go
//	internal/model/model.go
//	internal/model/ledger.go
//	internal/store/store.go
//	node_modules/pkg/index.js   (uncovered: default exclude)
//	.git/config                 (uncovered: hidden)
//	changes/ledger.md           (uncovered: workflow tree)
//	STRUCTURE.md, AGENTS.md     (doc files: excluded from entries)
//	.hiddenfile
func mkTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"README.md",
		"cmd/main.go",
		"internal/model/model.go",
		"internal/model/ledger.go",
		"internal/store/store.go",
		"node_modules/pkg/index.js",
		".git/config",
		"changes/ledger.md",
		"STRUCTURE.md",
		"AGENTS.md",
		".hiddenfile",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWalk(t *testing.T) {
	root := mkTree(t)
	cfg := DefaultConfig()
	d, err := Walk(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if d.Rel != "." {
		t.Errorf("root rel: %q", d.Rel)
	}
	if got := d.Files; len(got) != 1 || got[0] != "README.md" {
		t.Errorf("root files: %v (doc files and hidden files must be excluded)", got)
	}
	if got := d.Subdirs; len(got) != 2 || got[0] != "cmd" || got[1] != "internal" {
		t.Errorf("root subdirs: %v", got)
	}
	if len(d.Children) != 2 {
		t.Fatalf("children: %d", len(d.Children))
	}
	internal := d.Children[1]
	if internal.Rel != "internal" {
		t.Errorf("child rel: %q", internal.Rel)
	}
	if got := internal.Subdirs; len(got) != 2 || got[0] != "model" || got[1] != "store" {
		t.Errorf("internal subdirs: %v", got)
	}
	model := internal.Children[0]
	if got := model.Files; len(got) != 2 || got[0] != "ledger.go" || got[1] != "model.go" {
		t.Errorf("model files (must be sorted): %v", got)
	}
}

func TestWalkSymlinkNotFollowed(t *testing.T) {
	root := mkTree(t)
	if err := os.Symlink(filepath.Join(root, "internal"), filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	d, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range d.Files {
		if f == "link" {
			return
		}
	}
	t.Errorf("symlink should appear as a file entry, files: %v subdirs: %v", d.Files, d.Subdirs)
}

func TestPostOrder(t *testing.T) {
	root := mkTree(t)
	d, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, x := range PostOrder(d) {
		order = append(order, x.Rel)
	}
	want := []string{"cmd", "internal/model", "internal/store", "internal", "."}
	if len(order) != len(want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v, want %v", order, want)
		}
	}
}
