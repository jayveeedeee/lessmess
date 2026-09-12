package docs

import (
	"os"
	"path/filepath"
	"strings"
)

// Doc file names managed by this package. They are excluded from entry
// listings: a folder does not document its own docs.
const (
	StructureFile = "STRUCTURE.md"
	AgentsFile    = "AGENTS.md"
)

// Dir is a covered directory's inventory for STRUCTURE.md generation.
type Dir struct {
	Rel      string   // slash-separated repo-relative path; "." for the root
	Abs      string   // absolute path
	Files    []string // base names of files (hidden files and doc files excluded)
	Subdirs  []string // base names of covered child directories
	Children []*Dir   // covered child dirs, aligned with Subdirs

	Purpose string // one-line summary, filled by Build; used by the parent's rollup
	Hash    string // covered-subtree hash, filled by Build
}

// Walk inventories the covered tree rooted at root. Uncovered directories are
// not descended into, and symlinks are never followed (a symlink appears as a
// file entry). os.ReadDir returns entries sorted by name, so Files, Subdirs,
// and Children are sorted without extra work.
func Walk(root string, cfg *Config) (*Dir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return walk(abs, ".", cfg)
}

func walk(abs, rel string, cfg *Config) (*Dir, error) {
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	d := &Dir{Rel: rel, Abs: abs}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // hidden files and dirs
		}
		if e.IsDir() {
			childRel := name
			if rel != "." {
				childRel = rel + "/" + name
			}
			if !cfg.Covered(childRel) {
				continue
			}
			child, err := walk(filepath.Join(abs, name), childRel, cfg)
			if err != nil {
				return nil, err
			}
			d.Subdirs = append(d.Subdirs, name)
			d.Children = append(d.Children, child)
			continue
		}
		if name == StructureFile || name == AgentsFile {
			continue
		}
		d.Files = append(d.Files, name)
	}
	return d, nil
}

// PostOrder returns d and its covered descendants, children before parents —
// the order in which Build must run so parents can embed child rollups.
func PostOrder(d *Dir) []*Dir {
	var out []*Dir
	var rec func(*Dir)
	rec = func(x *Dir) {
		for _, c := range x.Children {
			rec(c)
		}
		out = append(out, x)
	}
	rec(d)
	return out
}
