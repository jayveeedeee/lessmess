package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasktracker/internal/model"
)

var testMeta = model.DocMeta{Refreshed: "2026-09-12", Source: "seed"}

// buildAll runs Build over the whole tree in PostOrder, writing each file
// through model.MergeDoc like real callers do.
func buildAll(t *testing.T, root *Dir) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, d := range PostOrder(root) {
		var existing []byte
		if b, ok := out[d.Rel]; ok {
			existing = []byte(b)
		} else {
			b, err := os.ReadFile(filepath.Join(root.Abs, filepath.FromSlash(d.Rel), StructureFile))
			if err == nil {
				existing = b
			}
		}
		auto, err := Build(d, existing, testMeta)
		if err != nil {
			t.Fatalf("Build %s: %v", d.Rel, err)
		}
		merged, err := model.MergeDoc(d.Rel+"/"+StructureFile, existing, []byte(auto))
		if err != nil {
			t.Fatalf("MergeDoc %s: %v", d.Rel, err)
		}
		out[d.Rel] = string(merged)
	}
	return out
}

func TestBuildGolden(t *testing.T) {
	root := mkTree(t)
	d, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	files := buildAll(t, d)

	leaf := files["internal/model"]
	wantLeaf := model.DocMarkerBegin + "\n" +
		"# Structure: internal/model\n\n" +
		"<!-- tasktracker-meta: refreshed=2026-09-12 source=seed tree=" + hashOf(t, d, "internal/model") + " -->\n\n" +
		"—\n" +
		"\n## Entries\n\n" +
		"| Entry | Purpose |\n| --- | --- |\n" +
		"| `ledger.go` | — |\n" +
		"| `model.go` | — |\n" +
		model.DocMarkerEnd + "\n"
	if leaf != wantLeaf {
		t.Errorf("leaf mismatch:\n--- got ---\n%s\n--- want ---\n%s", leaf, wantLeaf)
	}

	parent := files["internal"]
	if !strings.Contains(parent, "| `model/` | — |") {
		t.Error("subdir row must quote child purpose (placeholder here)")
	}
	if !strings.Contains(parent, "| `store/` | — |") {
		t.Error("missing store subdir row")
	}

	rootDoc := files["."]
	if !strings.Contains(rootDoc, "# Structure: "+filepath.Base(d.Abs)) {
		t.Error("root title must be the repo dir base name")
	}
	if strings.Contains(rootDoc, "node_modules") || strings.Contains(rootDoc, "changes") || strings.Contains(rootDoc, "STRUCTURE.md") {
		t.Error("root doc must not mention uncovered dirs or doc files")
	}
}

func hashOf(t *testing.T, root *Dir, rel string) string {
	t.Helper()
	for _, d := range PostOrder(root) {
		if d.Rel == rel {
			if d.Hash == "" {
				t.Fatal("hash not stamped; Build order wrong")
			}
			return d.Hash
		}
	}
	t.Fatalf("dir %s not found", rel)
	return ""
}

func TestBuildIdempotent(t *testing.T) {
	root := mkTree(t)
	d, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	first := buildAll(t, d)

	// Write the first pass to disk, then re-walk and rebuild from disk.
	for rel, content := range first {
		p := filepath.Join(root, filepath.FromSlash(rel), StructureFile)
		if rel == "." {
			p = filepath.Join(root, StructureFile)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d2, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	second := buildAll(t, d2)
	for rel, a := range first {
		if second[rel] != a {
			t.Errorf("%s: second build differs:\n--- first ---\n%s\n--- second ---\n%s", rel, a, second[rel])
		}
	}
}

func TestBuildCarryForward(t *testing.T) {
	root := mkTree(t)
	d, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	first := buildAll(t, d)

	// Simulate the LLM/human pass: fill in purpose and a blurb on the leaf.
	leaf := first["internal/model"]
	leaf = strings.Replace(leaf, "\n—\n", "\nParsers and serializers for the workflow file formats.\n", 1)
	leaf = strings.Replace(leaf, "| `model.go` | — |", "| `model.go` | Core status types |", 1)
	p := filepath.Join(root, "internal", "model", StructureFile)
	if err := os.WriteFile(p, []byte(leaf), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add a new file: tree changes, annotations must survive, new entry gets
	// a placeholder, hash and meta advance.
	if err := os.WriteFile(filepath.Join(root, "internal", "model", "table.go"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d2, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	second := buildAll(t, d2)
	got := second["internal/model"]
	if !strings.Contains(got, "Parsers and serializers for the workflow file formats.") {
		t.Error("purpose not carried forward")
	}
	if !strings.Contains(got, "| `model.go` | Core status types |") {
		t.Error("blurb not carried forward")
	}
	if !strings.Contains(got, "| `table.go` | — |") {
		t.Error("new file must get placeholder")
	}
	if !strings.Contains(got, "| `ledger.go` | — |") {
		t.Error("untouched blurb must remain placeholder")
	}
	if hashOf(t, d2, "internal/model") == hashOf(t, d, "internal/model") {
		t.Error("hash must change when entries change")
	}

	// The parent's rollup must quote the child's new purpose.
	parent := second["internal"]
	if !strings.Contains(parent, "| `model/` | Parsers and serializers for the workflow file formats. |") {
		t.Errorf("rollup must quote child purpose:\n%s", parent)
	}
	// Root tree changed too (descendant hash), so root hash advances.
	if hashOf(t, d2, ".") == hashOf(t, d, ".") {
		t.Error("root hash must change when a descendant changes")
	}
}

func TestBuildHashStable(t *testing.T) {
	root := mkTree(t)
	d1, _ := Walk(root, DefaultConfig())
	buildAll(t, d1)
	// Touch file *content* only: structure unchanged, hash unchanged.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("different content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d2, _ := Walk(root, DefaultConfig())
	buildAll(t, d2)
	if hashOf(t, d1, ".") != hashOf(t, d2, ".") {
		t.Error("content-only change must not change the tree hash")
	}
}

func TestBuildCorruptExisting(t *testing.T) {
	root := mkTree(t)
	p := filepath.Join(root, "cmd", StructureFile)
	if err := os.WriteFile(p, []byte("x\n"+model.DocMarkerEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Walk(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range PostOrder(d) {
		if x.Rel != "cmd" {
			continue
		}
		existing, _ := os.ReadFile(p)
		if _, err := Build(x, existing, testMeta); err == nil {
			t.Fatal("corrupt markers in existing file must be an error")
		}
	}
}
