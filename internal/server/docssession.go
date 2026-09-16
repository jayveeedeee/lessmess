package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/model"
	"lessmess/internal/opencode"
)

// gardenerRunner executes queued docs jobs with one unattended opencode
// session ("doc gardener") per job, guarded by a snapshot/verify/restore
// confinement check. It implements DocsRunner. spawn, when set, replaces
// plain session creation so configured agent/model defaults apply (wired
// by SetOpencode); nil spawn falls back to oc.CreateSession.
type gardenerRunner struct {
	oc    docs.SessionClient
	root  string
	cfg   *docs.Config
	spawn func(ctx context.Context, title string) (*opencode.Session, error)
}

// RunDocsJob refreshes the doc pairs for the job's directories:
//  1. deterministic skeleton refresh for the whole covered tree (structure is
//     machine-owned; changed trees are stamped with the job's change ID),
//  2. snapshot the job dirs' doc files (Dirs are update targets, Ancestors
//     are review-and-fix targets — both get the full cage),
//  3. one gardener session annotates placeholders, amends learnings, and
//     corrects or deletes ancestor learnings the change invalidated,
//  4. verify confinement (only auto sections may differ; meta hashes intact);
//     violations restore the snapshots and fail the job,
//  5. settle rollups so parents quote freshly written purposes.
func (r *gardenerRunner) RunDocsJob(ctx context.Context, job DocsJob) error {
	meta := model.DocMeta{Refreshed: time.Now().Format("2006-01-02"), Source: job.Change}
	if _, err := docs.RefreshSkeletons(r.root, r.cfg, meta); err != nil {
		return fmt.Errorf("skeleton refresh: %w", err)
	}

	// Snapshot the job dirs that exist (a change may have deleted one).
	tree, err := docs.Walk(r.root, r.cfg)
	if err != nil {
		return err
	}
	hashes, err := docs.TreeHashes(r.root, r.cfg)
	if err != nil {
		return err
	}
	byRel := map[string]*docs.Dir{}
	for _, d := range docs.PostOrder(tree) {
		byRel[d.Rel] = d
	}
	snap := map[string][]byte{}
	snapDir := func(rel string) *docs.Dir {
		d, ok := byRel[rel]
		if !ok {
			slog.Info("docs job dir no longer covered; skipping", "dir", rel, "change", job.Change)
			return nil
		}
		for _, f := range []string{docs.StructureFile, docs.AgentsFile} {
			p := filepath.Join(d.Abs, f)
			if data, err := os.ReadFile(p); err == nil {
				snap[p] = data
			}
		}
		return d
	}
	var update, review []*docs.Dir
	for _, rel := range job.Dirs {
		if d := snapDir(rel); d != nil {
			update = append(update, d)
		}
	}
	for _, rel := range job.Ancestors {
		if d := snapDir(rel); d != nil {
			review = append(review, d)
		}
	}
	targets := append(append([]*docs.Dir{}, update...), review...)

	if len(targets) > 0 {
		if err := r.garden(ctx, job, update, review); err != nil {
			r.restore(snap, targets)
			return err
		}
		for _, d := range targets {
			if err := verifyGardener(d, snap, hashes[d.Rel]); err != nil {
				slog.Warn("gardener confinement violation; reverting", "dir", d.Rel, "err", err)
				r.restore(snap, targets)
				return fmt.Errorf("gardener confinement violation in %s: %w", d.Rel, err)
			}
		}
	}

	if _, err := docs.RefreshSkeletons(r.root, r.cfg, meta); err != nil {
		return fmt.Errorf("rollup settle: %w", err)
	}
	return nil
}

// garden creates, primes, awaits, and cleans up one gardener session.
func (r *gardenerRunner) garden(ctx context.Context, job DocsJob, update, review []*docs.Dir) error {
	var sess *opencode.Session
	var err error
	if r.spawn != nil {
		sess, err = r.spawn(ctx, job.Change+" — docs")
	} else {
		sess, err = r.oc.CreateSession(ctx, job.Change+" — docs", r.root)
	}
	if err != nil {
		return fmt.Errorf("create gardener session: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = r.oc.DeleteSession(ctx, sess.ID)
	}()
	if err := r.oc.Prompt(ctx, sess.ID, appendAddendum(gardenerPrompt(job, update, review), promptAddendumFor(r.root, "gardener"))); err != nil {
		return fmt.Errorf("prompt gardener: %w", err)
	}
	if err := r.oc.WaitDone(ctx, sess.ID); err != nil {
		return fmt.Errorf("wait gardener: %w", err)
	}
	return nil
}

// restore rolls the target dirs' doc files back to their pre-session state:
// snapshotted files are rewritten; a file the session created (no snapshot)
// is removed — the one sanctioned deletion, rolling back an unreviewed
// creation.
func (r *gardenerRunner) restore(snap map[string][]byte, targets []*docs.Dir) {
	for p, data := range snap {
		if err := model.WriteFileAtomic(p, data, 0o644); err != nil {
			slog.Error("gardener restore", "file", p, "err", err)
		}
	}
	for _, d := range targets {
		for _, f := range []string{docs.StructureFile, docs.AgentsFile} {
			p := filepath.Join(d.Abs, f)
			if _, ok := snap[p]; ok {
				continue
			}
			if _, err := os.Stat(p); err == nil {
				if err := os.Remove(p); err != nil {
					slog.Error("gardener restore: remove created file", "file", p, "err", err)
				}
			}
		}
	}
}

// verifyGardener checks one target dir after the session: both doc files
// still parse, everything outside the markers is byte-identical to the
// snapshot, STRUCTURE.md's meta hash still matches the walked tree hash, no
// snapshot file was deleted, and a created AGENTS.md actually uses markers.
func verifyGardener(d *docs.Dir, snap map[string][]byte, treeHash string) error {
	for _, f := range []string{docs.StructureFile, docs.AgentsFile} {
		p := filepath.Join(d.Abs, f)
		before, existed := snap[p]
		after, err := os.ReadFile(p)
		if err != nil {
			if existed && os.IsNotExist(err) {
				return fmt.Errorf("%s was deleted", f)
			}
			continue // still absent; fine
		}
		name := d.Rel + "/" + f
		postDoc, err := model.ParseDocFile(name, after)
		if err != nil {
			return err
		}
		if !existed {
			if f == docs.AgentsFile && !postDoc.HasAuto {
				return fmt.Errorf("created AGENTS.md has no tasktracker marker section")
			}
			continue
		}
		preDoc, err := model.ParseDocFile(name, before)
		if err != nil {
			return err
		}
		if preDoc.Prefix != postDoc.Prefix || preDoc.Suffix != postDoc.Suffix {
			return fmt.Errorf("%s: content outside the tasktracker markers changed", f)
		}
		if f == docs.StructureFile {
			meta, err := model.ParseDocMeta(name, postDoc.Auto)
			if err != nil {
				return err
			}
			if meta == nil || meta.TreeHash != treeHash {
				return fmt.Errorf("STRUCTURE.md meta missing or tree hash altered")
			}
		}
	}
	return nil
}

// Shared gardener prompt fragments: the structure-annotation step, the
// preserve rules, and the scope limits are identical for change jobs and
// manual reconciliations.
const (
	gardenerStructureStep = "2. STRUCTURE.md was just regenerated and reflects the current tree. Between the `<!-- tasktracker:begin -->` and `<!-- tasktracker:end -->` markers, replace every em dash placeholder (—) purpose with a concise phrase. Do NOT add, remove, reorder, or rename entries; do NOT touch the tasktracker-meta line, covered-subdirectory rows, the heading, or the table structure; do NOT use the characters | or newlines inside table cells."

	gardenerHardRules = `HARD RULES: the file must end up with EXACTLY ONE begin marker and ONE end marker — never write these marker strings anywhere else (no prose mentions, no examples). Preserve all human-written content outside the markers byte for byte; do not reorganize, reformat, or "improve" it. Learnings are current-state facts about how the code works now: never narrate a change, never prefix with a change ID or a source tag — attribution lives in git history. Keep existing learnings unless one is now wrong or superseded; fix, merge, or delete those in place. Keep the section at or under 15 learnings; when a change would push it past that, merge or prune the weakest entries first.`

	gardenerScopeStep = "4. Do not modify anything outside the `<!-- tasktracker:begin -->`/`<!-- tasktracker:end -->` markers of those two files in the listed directories. Do not modify any other file. Do not run git commands."
)

// quotedRefs renders lint-flagged references as backticked list items.
func quotedRefs(refs []string) string {
	quoted := make([]string, 0, len(refs))
	for _, r := range refs {
		quoted = append(quoted, "`"+r+"`")
	}
	return strings.Join(quoted, ", ")
}

// lintRefsSection renders the manual job's flagged-reference block, empty
// when the job carries none.
func lintRefsSection(job DocsJob, update []*docs.Dir) string {
	if len(job.LintRefs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nA reference lint flagged paths cited by learnings that no longer exist. Fix or delete the learnings citing them — deleting a learning that was purely about a missing path is expected:\n")
	for _, d := range update {
		refs := job.LintRefs[d.Rel]
		if len(refs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s\n", d.Rel, quotedRefs(refs))
	}
	return b.String()
}

// gardenerPrompt builds the session prompt. Change jobs get two sections:
// update dirs (annotate placeholders, consolidate learnings) and
// review-and-fix dirs (ancestors, where the change may have invalidated
// learnings — fixing or deleting them is explicitly licensed and must be
// accounted for in the reply). A "manual" job is a reconciliation with no
// change record: one general review section, plus any lint-flagged
// references. Pure function, table-tested.
func gardenerPrompt(job DocsJob, update, review []*docs.Dir) string {
	var dirs strings.Builder
	for _, d := range update {
		fmt.Fprintf(&dirs, "- %s\n", d.Rel)
	}

	var b strings.Builder
	if job.Change == "manual" {
		b.WriteString(`You are the doc gardener for this repository. This is a MANUAL reconciliation: no change record exists for it.

Update the agent-facing docs for EXACTLY these directories, and no others:
`)
		b.WriteString(dirs.String())
		b.WriteString(`For each listed directory:

1. Skim the directory (and its covered children's STRUCTURE.md files) to understand its current state.
`)
		b.WriteString(gardenerStructureStep)
		b.WriteString("\n3. AGENTS.md: consolidate the auto section inside the markers based on your review — add only durable knowledge an agent here would not get from the code alone, reword or delete entries that no longer hold, phrase every learning as how the code works now (never change narration, no provenance prefixes), and keep the section at or under 15 learnings. ")
		b.WriteString(gardenerHardRules)
		b.WriteString("\n")
		b.WriteString(gardenerScopeStep)
		b.WriteString(lintRefsSection(job, update))
		b.WriteString("\n\nReply with one line per directory summarizing what you updated.")
		return b.String()
	}

	fmt.Fprintf(&b, `You are the doc gardener for this repository. A change has just been completed:

Change: %[1]s — %[2]s

Update the agent-facing docs for EXACTLY these directories, and no others:
%[3]s
For each listed directory:

1. Read the change record at changes/%[1]s/ (plan.md, ledger.md, task files) and skim the directory to understand what the change did.
`, job.Change, job.Title, dirs.String())
	b.WriteString(gardenerStructureStep)
	b.WriteString("\n3. AGENTS.md: consolidate the auto section inside the markers around what THIS change taught:\n   - Add a learning only for durable knowledge an agent working in this directory would not get from the code alone.\n   - Reword, merge, or delete existing learnings the change superseded — leaving stale text behind is a failure.\n   - Phrase every learning as how the code works NOW; never narrate the change and add no provenance prefixes.\n   - Keep the section at or under 15 learnings. ")
	b.WriteString(gardenerHardRules)
	b.WriteString("\n")
	b.WriteString(gardenerScopeStep)

	if len(review) > 0 {
		var rev strings.Builder
		for _, d := range review {
			fmt.Fprintf(&rev, "- %s\n", d.Rel)
		}
		fmt.Fprintf(&b, `

Then REVIEW-AND-FIX these directories — ancestors of the update list that may hold learnings about the change's surface:
%[1]s
For each review directory, read the change record first, then check every existing AGENTS.md learning against what this change removed or changed:
- If a learning references files, commands, symbols, or behavior that this change removed or materially changed, fix it in place — or DELETE it when the learning was purely about the removed surface. Deleting such a learning is expected and correct.
- Add a learning only if the change taught something at that directory's level (conventions or facts spanning its children), phrased as a current-state fact with no provenance prefix.
- Do not touch STRUCTURE.md in review directories: the deterministic pass already refreshed it. The same hard rules and scope limits apply to every edit.
`, rev.String())
		b.WriteString("\nYour reply must account for your work: one line per directory (both lists) summarizing what you updated, plus one line per learning you removed or edited, naming it and why.")
	} else {
		b.WriteString("\nReply with one line per directory summarizing what you updated.")
	}
	return b.String()
}
