package docs

import (
	"os"
	"path/filepath"
	"testing"

	"tasktracker/internal/model"
)

func TestDirDocs(t *testing.T) {
	root := t.TempDir()
	d := &Dir{Rel: ".", Abs: root}

	// Missing file: zero values, no error.
	c, err := DirDocs(d)
	if err != nil || c.Purpose != "" || len(c.Blurbs) != 0 || c.Meta != nil {
		t.Errorf("missing file: %+v, %v", c, err)
	}

	// Annotated file: purpose, blurbs (placeholders omitted), meta.
	content := model.DocMarkerBegin + "\n# Structure: x\n\n" +
		"<!-- tasktracker-meta: refreshed=2026-09-12 source=seed tree=abc123 -->\n\n" +
		"Parses things.\n\n## Entries\n\n| Entry | Purpose |\n| --- | --- |\n" +
		"| `a.go` | does A |\n| `b.go` | " + Placeholder + " |\n" + model.DocMarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(root, StructureFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err = DirDocs(d)
	if err != nil {
		t.Fatal(err)
	}
	if c.Purpose != "Parses things." {
		t.Errorf("purpose: %q", c.Purpose)
	}
	if len(c.Blurbs) != 1 || c.Blurbs["a.go"] != "does A" {
		t.Errorf("blurbs: %v", c.Blurbs)
	}
	if c.Meta == nil || c.Meta.TreeHash != "abc123" {
		t.Errorf("meta: %+v", c.Meta)
	}

	// Placeholder purpose: reported as empty.
	content2 := model.DocMarkerBegin + "\n# Structure: x\n\n" + Placeholder + "\n" + model.DocMarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(root, StructureFile), []byte(content2), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err = DirDocs(d)
	if err != nil || c.Purpose != "" {
		t.Errorf("placeholder purpose: %q, %v", c.Purpose, err)
	}

	// Corrupt markers: error.
	if err := os.WriteFile(filepath.Join(root, StructureFile), []byte("x\n"+model.DocMarkerEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DirDocs(d); err == nil {
		t.Error("corrupt markers must error")
	}
}
