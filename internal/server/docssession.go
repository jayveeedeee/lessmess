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
//  2. snapshot the job dirs' doc files,
//  3. one gardener session annotates placeholders and amends learnings,
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
	var targets []*docs.Dir
	snap := map[string][]byte{}
	for _, rel := range job.Dirs {
		d, ok := byRel[rel]
		if !ok {
			slog.Info("docs job dir no longer covered; skipping", "dir", rel, "change", job.Change)
			continue
		}
		targets = append(targets, d)
		for _, f := range []string{docs.StructureFile, docs.AgentsFile} {
			p := filepath.Join(d.Abs, f)
			if data, err := os.ReadFile(p); err == nil {
				snap[p] = data
			}
		}
	}

	if len(targets) > 0 {
		if err := r.garden(ctx, job, targets); err != nil {
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
func (r *gardenerRunner) garden(ctx context.Context, job DocsJob, targets []*docs.Dir) error {
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
	if err := r.oc.Prompt(ctx, sess.ID, appendAddendum(gardenerPrompt(job, targets), promptAddendumFor(r.root, "gardener"))); err != nil {
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

// gardenerPrompt builds the session prompt: change context, target dirs, and
// the write constraints. A "manual" job is a reconciliation with no change
// record: the gardener does a general review instead. Pure function,
// table-tested.
func gardenerPrompt(job DocsJob, targets []*docs.Dir) string {
	var dirs strings.Builder
	for _, d := range targets {
		fmt.Fprintf(&dirs, "- %s\n", d.Rel)
	}
	context := fmt.Sprintf(`You are the doc gardener for this repository. A change has just been completed:

Change: %[1]s — %[2]s`, job.Change, job.Title)
	step1 := `1. Read the change record at changes/%[1]s/ (plan.md, ledger.md, task files) and skim the directory to understand what the change did.`
	learnings := `3. AGENTS.md: add or refine 1–5 brief learnings inside the markers that would help an agent working in this directory, based on THIS change. Prefix each new learning with the change ID in parentheses, e.g. "(%[1]s) ...".`
	if job.Change == "manual" {
		context = `You are the doc gardener for this repository. This is a MANUAL reconciliation: no change record exists for it.`
		step1 = `1. Skim the directory (and its covered children's STRUCTURE.md files) to understand its current state.`
		learnings = `3. AGENTS.md: add or refine 1–5 brief learnings inside the markers that would help an agent working in this directory, based on your review. Prefix each new learning with "(manual)".`
	}
	return fmt.Sprintf(`%[1]s

Update the agent-facing docs for EXACTLY these directories, and no others:
%[2]s
For each listed directory:

%[3]s
2. STRUCTURE.md was just regenerated and reflects the current tree. Between the `+"`<!-- tasktracker:begin -->`"+` and `+"`<!-- tasktracker:end -->`"+` markers, replace every em dash placeholder (—) purpose with a concise phrase. Do NOT add, remove, reorder, or rename entries; do NOT touch the tasktracker-meta line, covered-subdirectory rows, the heading, or the table structure; do NOT use the characters | or newlines inside table cells.
%[4]s HARD RULES: the file must end up with EXACTLY ONE begin marker and ONE end marker — never write these marker strings anywhere else (no prose mentions, no examples). Preserve all human-written content outside the markers byte for byte; do not reorganize, reformat, or "improve" it. Keep existing learnings unless one is now wrong; fix those in place.
4. Do not modify anything outside the `+"`<!-- tasktracker:begin -->`"+`/`+"`<!-- tasktracker:end -->`"+` markers of those two files in the listed directories. Do not modify any other file. Do not run git commands.

Reply with one line per directory summarizing what you updated.`, context, dirs.String(), fmt.Sprintf(step1, job.Change), fmt.Sprintf(learnings, job.Change))
}
