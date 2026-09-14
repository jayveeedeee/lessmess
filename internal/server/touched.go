package server

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"lessmess/internal/docs"
)

// touchedDocsDirs computes the covered directories a change touched, from the
// "Files affected" sections of its task files. Heuristic, documented in the
// DOC-05 task file: bullet tokens that look like repo-relative paths are
// mapped to their nearest covered ancestor directory; prose is ignored.
//
// Excluded by design: paths under changes/ (closing a change always edits its
// own record — that must not trigger docs), hidden paths, and the doc files
// themselves (recursion exemption: gardener writes never re-enqueue).
func touchedDocsDirs(root, changeID string, cfg *docs.Config) ([]string, error) {
	tasksDir := filepath.Join(root, "changes", changeID, "tasks")
	ents, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tasksDir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, p := range filesAffected(string(data)) {
			if d, ok := coveredAncestor(p, cfg); ok {
				set[d] = true
			}
		}
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// filesAffected extracts candidate repo-relative paths from a markdown task
// body's "## Files affected" section.
func filesAffected(body string) []string {
	var out []string
	inSection := false
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			inSection = t == "## Files affected"
			continue
		}
		if !inSection {
			continue
		}
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "-"), "*"))
		if t == "" {
			continue
		}
		tok := strings.Fields(t)
		if len(tok) == 0 {
			continue
		}
		p := strings.Trim(tok[0], "`,;:")
		p = strings.TrimPrefix(p, "./")
		if p == "" || strings.HasPrefix(p, "http") || strings.HasPrefix(p, "#") {
			continue
		}
		if !strings.ContainsAny(p, "/.") {
			continue // prose word, not a path
		}
		out = append(out, p)
	}
	return out
}

// coveredAncestors returns, for each dir in dirs, every covered ancestor
// on its path to the root (the root included whenever a config exists),
// deduped within itself and against dirs, sorted. Uncovered intermediate
// segments are skipped but the walk continues bubbling toward the root —
// the same nearest-covered-ancestor escape hatch coveredAncestor uses
// downward. Purely lexical: deleted paths still produce ancestors.
func coveredAncestors(dirs []string, cfg *docs.Config) []string {
	primary := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		primary[d] = true
	}
	set := map[string]bool{}
	for _, d := range dirs {
		for d != "." {
			d = path.Dir(d)
			if !primary[d] && cfg.Covered(d) {
				set[d] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// coveredAncestor maps a touched path to the nearest covered directory that
// contains it. The root (".") is covered whenever a config exists.
func coveredAncestor(rel string, cfg *docs.Config) (string, bool) {
	isDir := strings.HasSuffix(rel, "/")
	rel = path.Clean(filepath.ToSlash(rel))
	if rel == "" || rel == "." {
		return "", false
	}
	if base := path.Base(rel); base == docs.StructureFile || base == docs.AgentsFile {
		return "", false
	}
	segs := strings.Split(rel, "/")
	for _, s := range segs {
		if strings.HasPrefix(s, ".") {
			return "", false
		}
	}
	if segs[0] == "changes" {
		return "", false
	}
	dir := path.Dir(rel)
	if isDir {
		dir = rel // the path itself is a directory
	}
	for {
		if dir == "." || dir == "" {
			return ".", true
		}
		if cfg.Covered(dir) {
			return dir, true
		}
		dir = path.Dir(dir)
	}
}
