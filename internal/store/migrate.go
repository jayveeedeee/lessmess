package store

// migrate.go needs slog for the MigrateIfNeeded log line.
import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"lessmess/internal/model"
)

// MigrateOptions configures the one-time markdown → JSON workflow
// migration.
type MigrateOptions struct {
	Dir        string         // repository root
	ChangeRoot ChangeRootFunc // optional worktree resolver for change directories
	DryRun     bool           // verify and report without writing anything
}

// MigrateResult summarizes a completed (or dry-run) migration.
type MigrateResult struct {
	Changes        int // change state files written
	Tasks          int // tasks collected (all depths)
	Containers     int // decomposed tasks (container ledgers migrated)
	FilesRewritten int // task prose files stripped of frontmatter
	LedgersDeleted int // markdown ledger files deleted (root + change + container)
}

// workflowPaths returns the canonical JSON state locations for a repo.
func workflowPaths(repoDir string) (indexFile, changesDir string) {
	wd := filepath.Join(repoDir, StateDirName, "workflow")
	return filepath.Join(wd, "index.json"), filepath.Join(wd, "changes")
}

// MigrateIfNeeded runs the markdown→JSON migration when the repository
// still has markdown workflow state (changes/ledger.md) and no JSON state
// yet. It reports whether a migration ran; a fresh repo (neither state)
// and an already-migrated one are both no-ops.
func MigrateIfNeeded(dir string, changeRoot ChangeRootFunc) (bool, error) {
	indexFile, _ := workflowPaths(dir)
	if _, err := os.Stat(indexFile); err == nil {
		return false, nil
	}
	if _, err := os.ReadFile(filepath.Join(dir, "changes", "ledger.md")); err != nil {
		return false, nil
	}
	res, err := MigrateWorkflow(MigrateOptions{Dir: dir, ChangeRoot: changeRoot})
	if err != nil {
		return false, err
	}
	slog.Info("workflow state migrated to JSON", "changes", res.Changes, "tasks", res.Tasks, "filesRewritten", res.FilesRewritten)
	return true, nil
}

// MigrateWorkflow converts a repository's markdown workflow state (root
// ledger, change ledgers, container ledgers, task frontmatter) into the
// JSON state store under .lessmess/workflow/ and deletes the markdown
// ledgers. It refuses to run when the JSON store already exists, and it
// verifies the collected state against the parsed source before writing
// or deleting anything; a failure leaves the tree untouched.
func MigrateWorkflow(opts MigrateOptions) (*MigrateResult, error) {
	repoDir := opts.Dir
	indexFile, changesDir := workflowPaths(repoDir)
	if _, err := os.Stat(indexFile); err == nil {
		return nil, fmt.Errorf("%s already exists: workflow state is already JSON (remove it to force a re-run)", indexFile)
	}
	rootData, err := os.ReadFile(filepath.Join(repoDir, "changes", "ledger.md"))
	if err != nil {
		return nil, fmt.Errorf("read root ledger: %w", err)
	}
	root, err := model.ParseRootLedger("changes/ledger.md", rootData)
	if err != nil {
		return nil, fmt.Errorf("parse root ledger: %w", err)
	}

	res := &MigrateResult{}
	idx := &model.WorkflowIndex{Version: model.StateVersion}
	states := make([]*model.ChangeState, 0, len(root.Rows))
	var proseRewrites []proseRewrite
	ledgerDeletes := []string{filepath.Join(repoDir, "changes", "ledger.md")}

	for _, row := range root.Rows {
		st, rewrites, deletes, err := collectChange(repoDir, row, opts.ChangeRoot, res)
		if err != nil {
			return nil, err
		}
		idx.Changes = append(idx.Changes, model.IndexEntry{
			ID:       row.Change,
			Title:    row.Title,
			Prefix:   unempty(row.Prefix),
			Branch:   unempty(row.Branch),
			Created:  row.Created,
			Archived: archivedRow(row),
		})
		states = append(states, st)
		proseRewrites = append(proseRewrites, rewrites...)
		ledgerDeletes = append(ledgerDeletes, deletes...)
	}

	if err := verifyMigration(root, idx, states); err != nil {
		return nil, fmt.Errorf("verification failed, nothing was written: %w", err)
	}
	if opts.DryRun {
		return res, nil
	}

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workflow dir: %w", err)
	}
	for _, st := range states {
		if err := st.Save(filepath.Join(changesDir, st.ID+".json")); err != nil {
			return nil, fmt.Errorf("write change state %s: %w", st.ID, err)
		}
	}
	if err := idx.Save(indexFile); err != nil {
		return nil, fmt.Errorf("write index: %w", err)
	}
	for _, rw := range proseRewrites {
		if err := model.WriteFileAtomic(rw.path, rw.body, 0o644); err != nil {
			return nil, fmt.Errorf("rewrite %s: %w", rw.path, err)
		}
		res.FilesRewritten++
	}
	for _, del := range ledgerDeletes {
		if err := os.Remove(del); err != nil {
			return nil, fmt.Errorf("delete %s: %w", del, err)
		}
		res.LedgersDeleted++
	}
	if err := patchGitignore(repoDir); err != nil {
		return nil, fmt.Errorf("update .gitignore: %w", err)
	}
	return res, nil
}

// proseRewrite is one task prose file to rewrite without its frontmatter.
type proseRewrite struct {
	path string
	body []byte
}

// levelRows holds one governing ledger's source rows for verification.
type levelRows struct {
	parent string
	rows   []model.TaskRow
}

// collectChange builds one change's JSON state from its markdown tree,
// verifying every level against its source ledger before returning. The
// returned slices are the prose rewrites and ledger deletions for it.
func collectChange(repoDir string, row model.RootRow, changeRoot ChangeRootFunc, res *MigrateResult) (*model.ChangeState, []proseRewrite, []string, error) {
	dir, ok := changeDir(repoDir, row.Change, changeRoot)
	if !ok {
		return nil, nil, nil, fmt.Errorf("change %s: directory not found under changes/ (or via worktree resolver)", row.Change)
	}
	ledgerPath := filepath.Join(dir, "ledger.md")
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change %s: read ledger: %w", row.Change, err)
	}
	ledger, err := model.ParseChangeLedger(path.Join("changes", row.Change, "ledger.md"), data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change %s: parse ledger: %w", row.Change, err)
	}
	if ledger.ChangeID != row.Change {
		return nil, nil, nil, fmt.Errorf("change %s: ledger header names %s", row.Change, ledger.ChangeID)
	}

	st := &model.ChangeState{
		Version:  model.StateVersion,
		ID:       row.Change,
		Title:    row.Title,
		Prefix:   unempty(row.Prefix),
		Branch:   unempty(row.Branch),
		Status:   model.ChangeStatus{Value: ledger.Overall, Derived: derivable(ledger.Overall)},
		Created:  row.Created,
		Updated:  row.Updated,
		Decision: parseDecisionLog(ledger.Lines),
	}
	var rewrites []proseRewrite
	deletes := []string{ledgerPath}
	levels := []levelRows{{parent: "", rows: ledger.Rows}}
	res.Changes++

	var walk func(absDir, relDir, parent string, rows []model.TaskRow) error
	walk = func(absDir, relDir, parent string, rows []model.TaskRow) error {
		entries, err := os.ReadDir(absDir)
		if err != nil {
			return fmt.Errorf("read %s: %w", relDir, err)
		}
		var names []string
		dirs := map[string]bool{}
		for _, e := range entries {
			if e.IsDir() {
				dirs[e.Name()] = true
			} else if strings.HasSuffix(e.Name(), ".md") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)

		// Row order is priority order; files without a row follow in
		// filename order (mirroring the store's tree scan).
		rowByBase := map[string]model.TaskRow{}
		var ordered []string
		seen := map[string]bool{}
		for i := range rows {
			base := path.Base(rows[i].Href)
			if !contains(names, base) {
				return fmt.Errorf("change %s: ledger row %s has no matching task file %s", row.Change, rows[i].ID, base)
			}
			if seen[base] {
				return fmt.Errorf("change %s: ledger row %s duplicates file %s", row.Change, rows[i].ID, base)
			}
			seen[base] = true
			rowByBase[base] = rows[i]
			ordered = append(ordered, base)
		}
		for _, n := range names {
			if !seen[n] {
				ordered = append(ordered, n)
			}
		}
		// Stray directories are validation violations; refuse to migrate.
		for d := range dirs {
			if !contains(names, d+".md") {
				return fmt.Errorf("change %s: %s%s is a stray directory (no matching task file)", row.Change, relDir, d)
			}
		}

		for _, name := range ordered {
			href := relDir + name
			fileData, err := os.ReadFile(filepath.Join(absDir, name))
			if err != nil {
				return fmt.Errorf("read %s: %w", href, err)
			}
			tf, err := model.ParseTaskFile(path.Join(row.Change, href), fileData)
			if err != nil {
				return fmt.Errorf("parse %s: %w", href, err)
			}
			r, hasRow := rowByBase[name]
			id := tf.ID
			if hasRow && r.ID != id {
				return fmt.Errorf("%s: frontmatter id %s does not match ledger row %s", href, id, r.ID)
			}
			seg := model.LastTaskSegment(id)
			seq, err := strconv.Atoi(seg)
			if err != nil {
				return fmt.Errorf("%s: cannot derive sequence from id %s", href, id)
			}
			ts := model.TaskState{
				ID:      id,
				Seq:     seq,
				Parent:  parent,
				Title:   tf.Title,
				File:    href,
				Status:  model.StatusNotStarted,
				Updated: row.Created, // fallback for tasks without a ledger row
			}
			if hasRow {
				ts.Title = r.Title
				ts.Status = r.Status
				ts.DependsOn = unemptyDeps(r.Depends)
				ts.Updated = r.Updated
				ts.Notes = unempty(r.Notes)
			}
			st.Tasks = append(st.Tasks, ts)
			res.Tasks++
			rewrites = append(rewrites, proseRewrite{path: filepath.Join(absDir, name), body: []byte(tf.Body)})

			base := strings.TrimSuffix(name, ".md")
			if dirs[base] {
				res.Containers++
				containerLedger := filepath.Join(absDir, base, "ledger.md")
				cdata, err := os.ReadFile(containerLedger)
				if err != nil {
					return fmt.Errorf("read container ledger %s%s/ledger.md: %w", relDir, base, err)
				}
				tl, err := model.ParseTaskLedger(path.Join(row.Change, relDir, base, "ledger.md"), cdata)
				if err != nil {
					return fmt.Errorf("parse container ledger %s%s/ledger.md: %w", relDir, base, err)
				}
				if tl.TaskID != id {
					return fmt.Errorf("%s%s/ledger.md: header names task %s, want %s", relDir, base, tl.TaskID, id)
				}
				deletes = append(deletes, containerLedger)
				levels = append(levels, levelRows{parent: id, rows: tl.Rows})
				if err := walk(filepath.Join(absDir, base, "tasks"), relDir+base+"/tasks/", id, tl.Rows); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(filepath.Join(dir, "tasks"), "tasks/", "", ledger.Rows); err != nil {
		return nil, nil, nil, err
	}

	// Per-level verification: the JSON tree must reproduce every source
	// row — same order, same fields — before anything is written.
	for _, lvl := range levels {
		tasks := st.Children(lvl.parent)
		if len(tasks) < len(lvl.rows) {
			return nil, nil, nil, fmt.Errorf("change %s: level %q collected %d tasks for %d ledger rows", st.ID, lvl.parent, len(tasks), len(lvl.rows))
		}
		for i, r := range lvl.rows {
			t := tasks[i]
			if t.ID != r.ID || t.Title != r.Title || t.Status != r.Status ||
				!equalDeps(t.DependsOn, unemptyDeps(r.Depends)) ||
				t.Updated != r.Updated || t.Notes != unempty(r.Notes) {
				return nil, nil, nil, fmt.Errorf("change %s: task %s does not round-trip its ledger row %+v (got %+v)", st.ID, r.ID, r, t)
			}
		}
	}
	return st, rewrites, deletes, nil
}

// verifyMigration cross-checks the collected index and states against the
// root ledger and the state schema. Per-level task verification happens
// inside collectChange.
func verifyMigration(root *model.RootLedger, idx *model.WorkflowIndex, states []*model.ChangeState) error {
	if len(idx.Changes) != len(root.Rows) {
		return fmt.Errorf("index has %d changes, root ledger has %d", len(idx.Changes), len(root.Rows))
	}
	for i, row := range root.Rows {
		st := states[i]
		if idx.Changes[i].ID != row.Change {
			return fmt.Errorf("index row %d is %s, want %s", i, idx.Changes[i].ID, row.Change)
		}
		if st.ID != row.Change {
			return fmt.Errorf("state %d is %s, want %s", i, st.ID, row.Change)
		}
		if st.Title != row.Title {
			return fmt.Errorf("change %s: title %q does not match root row %q", st.ID, st.Title, row.Title)
		}
		if issues := st.Validate(); len(issues) > 0 {
			return fmt.Errorf("change %s: %s", st.ID, strings.Join(issues, "; "))
		}
	}
	if issues := idx.Validate(); len(issues) > 0 {
		return fmt.Errorf("index: %s", strings.Join(issues, "; "))
	}
	return nil
}

// derivable reports whether an overall status is one the tool derives
// from the task tree (as opposed to user-set Done/Cancelled).
func derivable(s model.OverallStatus) bool {
	switch s {
	case model.OverallPlanned, model.OverallInProgress, model.OverallBlocked:
		return true
	}
	return false
}

// changeDir resolves a change's directory: main tree (active, then
// changes/archive/) first, then the worktree resolver.
func changeDir(repoDir, id string, changeRoot ChangeRootFunc) (string, bool) {
	for _, rel := range []string{
		filepath.Join("changes", id),
		filepath.Join("changes", "archive", id),
	} {
		if info, err := os.Stat(filepath.Join(repoDir, rel)); err == nil && info.IsDir() {
			return filepath.Join(repoDir, rel), true
		}
	}
	if changeRoot != nil {
		if dir, ok := changeRoot(id); ok {
			return dir, true
		}
	}
	return "", false
}

// archivedRow reports whether a root row points into changes/archive/
// (the href reveals it: "archive/<id>/plan.md").
func archivedRow(row model.RootRow) bool {
	return strings.HasPrefix(row.Href, "archive/")
}

// parseDecisionLog extracts the optional "| Date | Decision |" table
// under "## Decision log". It returns nil when the section or its table
// is absent.
func parseDecisionLog(lines []string) []model.DecisionEntry {
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "## Decision log") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	for i := start; i < len(lines); i++ {
		cells := tableCells(lines[i])
		if cells == nil {
			continue
		}
		if len(cells) == 2 && cells[0] == "Date" && cells[1] == "Decision" {
			var out []model.DecisionEntry
			for j := i + 1; j < len(lines); j++ {
				c := tableCells(lines[j])
				if c == nil || len(c) != 2 {
					break
				}
				if isSepRow(c) {
					continue
				}
				out = append(out, model.DecisionEntry{Date: c[0], Decision: c[1]})
			}
			return out
		}
	}
	return nil
}

// tableCells splits one markdown table line into trimmed cells; nil when
// the line is not a table row.
func tableCells(ln string) []string {
	t := strings.TrimSpace(ln)
	if len(t) < 2 || !strings.HasPrefix(t, "|") || !strings.HasSuffix(t, "|") {
		return nil
	}
	t = strings.TrimPrefix(strings.TrimSuffix(t, "|"), "|")
	parts := strings.Split(t, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSepRow(cells []string) bool {
	for _, c := range cells {
		t := strings.TrimLeft(c, ":")
		if !strings.HasPrefix(t, "-") {
			return false
		}
	}
	return true
}

func contains(names []string, s string) bool {
	for _, n := range names {
		if n == s {
			return true
		}
	}
	return false
}

func equalDeps(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// unempty maps the markdown empty cell ("—") to the JSON empty value ("").
func unempty(s string) string {
	if strings.TrimSpace(s) == model.Empty {
		return ""
	}
	return s
}

func unemptyDeps(deps []string) []string {
	var out []string
	for _, d := range deps {
		d = strings.TrimSpace(d)
		if d != "" && d != model.Empty {
			out = append(out, d)
		}
	}
	return out
}

// patchGitignore makes .lessmess/workflow/ committable: a whole-directory
// ignore (".lessmess/") becomes ".lessmess/*" plus a negation for the
// workflow subtree, because git cannot re-include files under an ignored
// directory. Idempotent.
func patchGitignore(repoDir string) error {
	gitignore := filepath.Join(repoDir, ".gitignore")
	data, err := os.ReadFile(gitignore)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(data), "\n")
	wantBlock := []string{".lessmess/*", "!.lessmess/workflow/"}
	hasBlock := false
	var out []string
	for _, ln := range lines {
		switch strings.TrimSpace(ln) {
		case ".lessmess/*":
			hasBlock = true
			out = append(out, ln)
		case "!.lessmess/workflow/":
			out = append(out, ln)
		case ".lessmess/", ".tasktracker/":
			if !hasBlock { // replace the directory ignore with the block once
				hasBlock = true
				out = append(out, wantBlock...)
			}
		default:
			out = append(out, ln)
		}
	}
	if !hasBlock {
		out = append(out, wantBlock...)
	}
	content := strings.Join(out, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return model.WriteFileAtomic(gitignore, []byte(content), 0o644)
}
