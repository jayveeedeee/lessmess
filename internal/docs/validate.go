package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"lessmess/internal/model"
)

// Finding severities for docs-system checks.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Finding is one docs-system check result. Structural problems (corrupt
// markers, unreadable metadata) are errors; coverage and freshness gaps are
// warnings — docs lag by design, so warnings never block anything.
type Finding struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Msg      string `json:"msg"`
}

// ValidateDocs checks docs-system health for the repo at root: covered dirs
// have parseable doc files, STRUCTURE.md freshness metadata matches the
// current tree, and stale dirs flagged by the server queue are reported
// (stale may be nil, e.g. from the CLI which has no live queue). It returns
// nil when the docs system is disabled (no agentsdocs.json).
func ValidateDocs(root string, stale map[string]string) []Finding {
	cfg, err := LoadConfig(root)
	if err != nil {
		return []Finding{{Severity: SeverityError, File: ConfigFile, Msg: err.Error()}}
	}
	if cfg == nil {
		return nil
	}
	var out []Finding
	tree, err := Walk(root, cfg)
	if err != nil {
		return []Finding{{Severity: SeverityError, File: ".", Msg: "walk: " + err.Error()}}
	}
	hashes, err := TreeHashes(root, cfg)
	if err != nil {
		return []Finding{{Severity: SeverityError, File: ".", Msg: "tree hashes: " + err.Error()}}
	}
	for _, d := range PostOrder(tree) {
		out = append(out, checkStructure(d, hashes[d.Rel])...)
		out = append(out, checkAgents(d)...)
	}
	out = append(out, staleRefFindings(root)...)
	for _, rel := range sortedKeys(stale) {
		out = append(out, Finding{SeverityWarning, joinRel(rel, StructureFile), "flagged stale by the docs queue: " + stale[rel]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Severity < out[j].Severity
	})
	return out
}

// checkStructure verifies one dir's STRUCTURE.md: existence, marker
// integrity, and freshness against the current tree hash.
func checkStructure(d *Dir, treeHash string) []Finding {
	rel := joinRel(d.Rel, StructureFile)
	data, err := os.ReadFile(filepath.Join(d.Abs, StructureFile))
	if os.IsNotExist(err) {
		return []Finding{{SeverityWarning, rel, "covered directory has no STRUCTURE.md (run 'lessmess docs seed')"}}
	}
	if err != nil {
		return []Finding{{SeverityError, rel, err.Error()}}
	}
	doc, err := model.ParseDocFile(rel, data)
	if err != nil {
		return []Finding{{SeverityError, rel, err.Error()}}
	}
	if !doc.HasAuto {
		return []Finding{{SeverityWarning, rel, "no tasktracker auto section"}}
	}
	meta, err := model.ParseDocMeta(rel, doc.Auto)
	switch {
	case err != nil:
		return []Finding{{SeverityError, rel, err.Error()}}
	case meta == nil:
		return []Finding{{SeverityWarning, rel, "freshness metadata missing"}}
	case meta.TreeHash != treeHash:
		return []Finding{{SeverityWarning, rel, "stale: tree changed since last refresh (close a change or run 'lessmess docs refresh')"}}
	}
	return nil
}

// checkAgents verifies one dir's AGENTS.md parses when present.
func checkAgents(d *Dir) []Finding {
	rel := joinRel(d.Rel, AgentsFile)
	data, err := os.ReadFile(filepath.Join(d.Abs, AgentsFile))
	if os.IsNotExist(err) {
		return []Finding{{SeverityWarning, rel, "covered directory has no AGENTS.md (run 'lessmess docs seed')"}}
	}
	if err != nil {
		return []Finding{{SeverityError, rel, err.Error()}}
	}
	if _, err := model.ParseDocFile(rel, data); err != nil {
		return []Finding{{SeverityError, rel, err.Error()}}
	}
	return nil
}

// staleRefFindings runs the stale-reference lint and renders its warnings.
// Identical missing paths across many directories are collapsed into one
// finding (a convention like "config lives in truservice.yaml, generated
// at deploy time" would otherwise produce one warning per service
// directory and bury the bell). A lint error is an error finding: the lint
// walks only files ValidateDocs already tolerates, so failure here means
// something unusual (config or walk trouble) that the user should see.
func staleRefFindings(root string) []Finding {
	staleRefs, err := StaleLearningRefs(root)
	if err != nil {
		return []Finding{{SeverityError, ConfigFile, "reference lint: " + err.Error()}}
	}
	type refHit struct {
		dirs []string
	}
	byRef := map[string][]string{}
	for rel, refs := range staleRefs {
		for _, ref := range refs {
			byRef[ref] = append(byRef[ref], rel)
		}
	}
	refs := make([]string, 0, len(byRef))
	for ref := range byRef {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	var out []Finding
	for _, ref := range refs {
		dirs := byRef[ref]
		sort.Strings(dirs)
		f := Finding{Severity: SeverityWarning, File: joinRel(dirs[0], AgentsFile)}
		if len(dirs) == 1 {
			f.Msg = fmt.Sprintf("learning cites missing path %q", ref)
		} else {
			f.Msg = fmt.Sprintf("learning cites missing path %q (%d directories, e.g. %s)", ref, len(dirs), dirs[0])
		}
		out = append(out, f)
	}
	return out
}

func joinRel(rel, name string) string {
	if rel == "." {
		return name
	}
	return rel + "/" + name
}

// StaleDirs returns the covered dirs whose STRUCTURE.md freshness metadata
// is missing/unparseable or lags the current tree hash — the dirs a refresh
// should reconcile. Missing STRUCTURE.md counts as stale. It returns nil
// when the docs system is disabled.
func StaleDirs(root string) ([]string, error) {
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
	hashes, err := TreeHashes(root, cfg)
	if err != nil {
		return nil, err
	}
	var stale []string
	for _, d := range PostOrder(tree) {
		data, err := os.ReadFile(filepath.Join(d.Abs, StructureFile))
		if err != nil {
			stale = append(stale, d.Rel)
			continue
		}
		doc, err := model.ParseDocFile(d.Rel+"/"+StructureFile, data)
		if err != nil || !doc.HasAuto {
			stale = append(stale, d.Rel)
			continue
		}
		meta, err := model.ParseDocMeta(d.Rel+"/"+StructureFile, doc.Auto)
		if err != nil || meta == nil || meta.TreeHash != hashes[d.Rel] {
			stale = append(stale, d.Rel)
		}
	}
	sort.Strings(stale)
	return stale, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
