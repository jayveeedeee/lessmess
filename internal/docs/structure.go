package docs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lessmess/internal/model"
)

// Placeholder marks a purpose or blurb that still needs a real description,
// written by the seed or gardener LLM pass.
const Placeholder = "—"

// Build renders the STRUCTURE.md auto section for d. existing is the current
// STRUCTURE.md content (nil if absent): the directory purpose line, file
// purpose blurbs, and — when the covered tree is unchanged — the freshness
// metadata are carried forward from it, so regenerating an unchanged tree
// yields byte-identical output. Covered subdir rows always quote the child's
// current Purpose (annotate children in their own STRUCTURE.md), which is how
// rollups compose upward. Children must be Built before their parent (see
// PostOrder). Build stamps d.Purpose and d.Hash.
func Build(d *Dir, existing []byte, meta model.DocMeta) (string, error) {
	purpose, blurbs, oldMeta, err := carryForward(d.Rel, existing)
	if err != nil {
		return "", err
	}
	d.Hash = treeHash(d)
	d.Purpose = purpose

	// Freshness only advances when the covered tree actually changed;
	// otherwise the previous stamp carries forward verbatim.
	if oldMeta != nil && oldMeta.TreeHash == d.Hash {
		meta = *oldMeta
	} else {
		meta.TreeHash = d.Hash
	}

	title := d.Rel
	if d.Rel == "." {
		title = filepath.Base(d.Abs)
	}
	var b strings.Builder
	b.WriteString("# Structure: " + title + "\n\n")
	b.WriteString(meta.String() + "\n\n")
	b.WriteString(purpose + "\n")
	if len(d.Subdirs) > 0 || len(d.Files) > 0 {
		b.WriteString("\n## Entries\n\n")
		b.WriteString("| Entry | Purpose |\n| --- | --- |\n")
		for i, s := range d.Subdirs {
			fmt.Fprintf(&b, "| `%s/` | %s |\n", s, cell(d.Children[i].Purpose))
		}
		for _, f := range d.Files {
			blurb := blurbs[f]
			if blurb == "" {
				blurb = Placeholder
			}
			fmt.Fprintf(&b, "| `%s` | %s |\n", f, cell(blurb))
		}
	}
	return b.String(), nil
}

// treeHash hashes the covered subtree's structure (entry names and kinds,
// recursively). File content is deliberately irrelevant: STRUCTURE.md
// documents structure, and content-driven learnings belong to the change
// close-out flow.
func treeHash(d *Dir) string {
	h := sha256.New()
	for _, f := range d.Files {
		fmt.Fprintf(h, "F %s\n", f)
	}
	for i, s := range d.Subdirs {
		fmt.Fprintf(h, "D %s %s\n", s, d.Children[i].Hash)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// carryForward extracts the human/LLM-written purpose line and file blurbs
// from an existing STRUCTURE.md, plus its freshness metadata. Missing file or
// missing markers yield placeholders; corrupt markers are an error.
func carryForward(rel string, existing []byte) (purpose string, blurbs map[string]string, meta *model.DocMeta, err error) {
	purpose = Placeholder
	blurbs = map[string]string{}
	if len(existing) == 0 {
		return purpose, blurbs, nil, nil
	}
	doc, err := model.ParseDocFile(rel+"/"+StructureFile, existing)
	if err != nil {
		return "", nil, nil, err
	}
	if !doc.HasAuto {
		return purpose, blurbs, nil, nil
	}
	meta, err = model.ParseDocMeta(rel+"/"+StructureFile, doc.Auto)
	if err != nil {
		return "", nil, nil, err
	}
	inEntries := false
	for _, line := range strings.Split(doc.Auto, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "" || strings.HasPrefix(t, "<!--"):
			continue
		case strings.HasPrefix(t, "# Structure"):
			continue
		case strings.HasPrefix(t, "## "):
			inEntries = t == "## Entries"
			continue
		case inEntries && strings.HasPrefix(t, "|"):
			name, blurb, ok := parseRow(t)
			if ok && blurb != Placeholder {
				blurbs[name] = blurb
			}
		case !inEntries && !strings.HasPrefix(t, "|") && purpose == Placeholder:
			purpose = t
		}
	}
	return purpose, blurbs, meta, nil
}

// parseRow parses one entries-table row ("| `name/` | blurb |") into name
// and blurb. Directory rows (trailing "/") are skipped: their blurbs come
// from the child Build, not from carry-forward.
func parseRow(row string) (name, blurb string, ok bool) {
	cells := strings.Split(row, "|")
	if len(cells) < 3 {
		return "", "", false
	}
	name = strings.Trim(strings.TrimSpace(cells[1]), "`")
	blurb = strings.TrimSpace(cells[2])
	if name == "" || name == "Entry" || strings.HasPrefix(name, "---") || strings.HasSuffix(name, "/") {
		return "", "", false
	}
	return name, blurb, true
}

// cell sanitizes text for a single markdown table cell.
func cell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "/")
}

// RefreshSkeletons rebuilds STRUCTURE.md for every covered directory,
// bottom-up, writing only files that changed. Directories whose tree hash
// changed are stamped with meta; unchanged trees keep their previous stamp
// (Build carry-forward). Returns the repo-relative dirs whose files changed.
// Used by the doc gardener on change close-out.
func RefreshSkeletons(root string, cfg *Config, meta model.DocMeta) ([]string, error) {
	tree, err := Walk(root, cfg)
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, d := range PostOrder(tree) {
		p := filepath.Join(d.Abs, StructureFile)
		existing, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return changed, err
		}
		auto, err := Build(d, existing, meta)
		if err != nil {
			return changed, err
		}
		merged, err := model.MergeDoc(d.Rel+"/"+StructureFile, existing, []byte(auto))
		if err != nil {
			return changed, err
		}
		if string(merged) == string(existing) {
			continue
		}
		if err := model.WriteFileAtomic(p, merged, 0o644); err != nil {
			return changed, err
		}
		changed = append(changed, d.Rel)
	}
	return changed, nil
}

// TreeHashes returns the covered-tree hash of every covered directory,
// without rendering or writing anything. Used by validation freshness checks.
func TreeHashes(root string, cfg *Config) (map[string]string, error) {
	tree, err := Walk(root, cfg)
	if err != nil {
		return nil, err
	}
	hashes := map[string]string{}
	for _, d := range PostOrder(tree) {
		d.Hash = treeHash(d)
		hashes[d.Rel] = d.Hash
	}
	return hashes, nil
}
