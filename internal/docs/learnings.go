package docs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lessmess/internal/model"
)

// StaleLearningRefs scans every covered directory's AGENTS.md auto section
// for backticked path-like references and returns, per repo-relative dir,
// the references that resolve to nothing in the repository. Resolution is
// deliberately generous — the reference may be repo-root-relative,
// dir-relative, exist anywhere in the tree by full or base name, or be a
// .lessmess tooling-state file — because learnings freely use short names
// for files that live in child directories ("settings.go" from internal/)
// and for gitignored state. Combined with the conservative shape rules in
// cleanRef this keeps precision high: a missed dead reference costs
// nothing today, while a false positive burns trust in the findings bell.
// Returns nil when the docs system is disabled.
func StaleLearningRefs(root string) (map[string][]string, error) {
	cfg, err := LoadConfig(root)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	tree, err := Walk(root, cfg)
	if err != nil {
		return nil, err
	}
	idx, err := newRepoIndex(root)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, d := range PostOrder(tree) {
		data, err := os.ReadFile(filepath.Join(d.Abs, AgentsFile))
		if err != nil {
			continue // absent AGENTS.md is validate's warning, not the lint's
		}
		doc, err := model.ParseDocFile(d.Rel+"/"+AgentsFile, data)
		if err != nil || !doc.HasAuto {
			continue // corrupt marker structure is validate's error, not the lint's
		}
		known := map[string]bool{}
		for _, f := range d.Files {
			known[f] = true
		}
		for _, s := range d.Subdirs {
			known[s] = true
		}
		var missing []string
		for _, ref := range pathLikeRefs(doc.Auto) {
			if known[ref] || idx.resolves(d.Rel, ref) {
				continue
			}
			if exists(filepath.Join(root, filepath.FromSlash(ref))) ||
				exists(filepath.Join(d.Abs, filepath.FromSlash(ref))) {
				continue
			}
			missing = append(missing, ref)
		}
		if len(missing) > 0 {
			out[d.Rel] = missing
		}
	}
	return out, nil
}

// repoIndex indexes every path under root (excluding .git and generated
// dependency dirs, hidden dirs included — .lessmess state is a resolution
// target) so references can be checked against full relative paths and
// bare base names without touching the disk per candidate.
type repoIndex struct {
	rels []string
	// basesList is the distinct set of file base names; it powers the
	// base-name and sibling-variant checks (truservice.yaml vs
	// truservice.yaml.template).
	basesList []string
}

func newRepoIndex(root string) (*repoIndex, error) {
	skipDir := func(name string) bool {
		if name == ".git" {
			return true
		}
		for _, d := range DefaultExclude {
			if d == name {
				return true
			}
		}
		return false
	}
	idx := &repoIndex{}
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if p != root && skipDir(e.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		idx.rels = append(idx.rels, rel)
		if !seen[e.Name()] {
			seen[e.Name()] = true
			idx.basesList = append(idx.basesList, e.Name())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(idx.rels)
	sort.Strings(idx.basesList)
	return idx, nil
}

// resolves reports whether ref names something that plausibly exists under
// root: as an exact relative path, as the tail of one ("settings.json"
// matching .lessmess/sessions.json-style state, "model.go" matching a
// file in some child directory), or as a **sibling variant** — any file
// whose name extends ref with a suffix ("truservice.yaml" alongside a
// "truservice.yaml.template" marks it a generated artifact that is
// legitimately absent from the repository).
func (x *repoIndex) resolves(dirRel, ref string) bool {
	base := filepath.Base(ref)
	for _, b := range x.basesList {
		if b == ref || b == base || strings.HasPrefix(b, base+".") {
			return true
		}
	}
	for _, rel := range x.rels {
		if rel == ref || rel == dirRel+"/"+ref || strings.HasSuffix(rel, "/"+ref) {
			return true
		}
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// pathLikeRefs extracts the path-shaped backticked spans of a learnings
// section, deduped and sorted.
func pathLikeRefs(auto string) []string {
	var out []string
	seen := map[string]bool{}
	for _, span := range backtickSpans(auto) {
		ref := cleanRef(span)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

// backtickSpans returns the contents of every `…` span in s.
func backtickSpans(s string) []string {
	var spans []string
	for {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			return spans
		}
		s = s[i+1:]
		j := strings.IndexByte(s, '`')
		if j < 0 {
			return spans
		}
		spans = append(spans, s[:j])
		s = s[j+1:]
	}
}

// cleanRef normalizes one backticked span into a checkable path, or ""
// when the span is not lintable: empty, whitespace-bearing (code snippet
// or prose), a URL, a marker string, carrying a hidden segment
// (dot-prefixed spans are usually shell fragments or dotfiles), carrying
// non-path characters (qualifiers like *model.Error or cmd/<name>/…), or
// not path-shaped. Trailing sentence punctuation is not part of the path.
func cleanRef(span string) string {
	ref := strings.TrimSpace(span)
	if ref == "" || strings.ContainsAny(ref, " \t\n") {
		return ""
	}
	ref = strings.TrimRight(ref, ",;:.")
	if ref == "" {
		return ""
	}
	lower := strings.ToLower(ref)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return ""
	}
	if strings.Contains(ref, model.DocMarkerBegin) || strings.Contains(ref, model.DocMarkerEnd) {
		return ""
	}
	// Qualifiers and placeholders, not paths: *model.Error, cmd/<name>/main.go.
	if strings.ContainsAny(ref, "*<>(){}[]!\"'") {
		return ""
	}
	for _, seg := range strings.Split(ref, "/") {
		if strings.HasPrefix(seg, ".") {
			return ""
		}
	}
	if !pathShaped(ref) {
		return ""
	}
	return ref
}

// knownExts is the allowlist of final-segment extensions that make a span
// a plausible file reference. This is the main symbol filter: Go-style
// qualified identifiers read like dotted names (server.New, store.Open,
// s.oc, filepath.Abs, Server.index, renderer.setup) but their tails are
// not file extensions, so they never lint. Add extensions sparingly —
// every entry is a chance to flag a symbol.
var knownExts = map[string]bool{
	"bash": true, "c": true, "cc": true, "cfg": true, "conf": true, "cpp": true,
	"cs": true, "css": true, "csv": true, "dart": true, "env": true, "go": true,
	"gif": true, "h": true, "hpp": true, "htm": true, "html": true, "ico": true,
	"ini": true, "java": true, "jpeg": true, "jpg": true, "js": true, "json": true,
	"jsx": true, "kt": true, "lock": true, "lua": true, "markdown": true, "md": true,
	"mjs": true, "mod": true, "pdf": true, "php": true, "pl": true, "png": true,
	"py": true, "rb": true,
	"rs": true, "sass": true, "scss": true, "sh": true, "sql": true, "svg": true,
	"sum": true, "swift": true, "toml": true, "ts": true, "tsv": true, "tsx": true,
	"txt": true, "wasm": true, "webp": true, "xml": true, "yaml": true, "yml": true,
	"zsh": true,
}

// pathShaped reports whether ref is checkable as a path: a dotted name
// with a known extension (app.js, settings.json) or a slashed path whose
// final segment carries one (internal/model/docfile.go). Unknown tails
// keep dotted identifiers ("Server.index"), model IDs
// ("provider/model-x"), version numbers ("v1"), and bare directory
// mentions ("internal/") out of the findings.
func pathShaped(ref string) bool {
	seg := ref
	if i := strings.LastIndexByte(ref, '/'); i >= 0 {
		seg = ref[i+1:]
	}
	dot := strings.LastIndexByte(seg, '.')
	if dot <= 0 { // no extension, or a leading dot (already filtered)
		return false
	}
	return knownExts[strings.ToLower(seg[dot+1:])]
}
