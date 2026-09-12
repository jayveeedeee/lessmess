package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tasktracker/internal/model"
)

// SeedOptions tunes a Seed run.
type SeedOptions struct {
	DryRun bool   // plan only: no writes, no summarizer calls
	Budget int    // max summarizer calls this run; 0 = unlimited
	Date   string // freshness date stamp; empty = today
}

// seedCursorPath is the resumable-seed state file (gitignored tooling state).
const seedCursorPath = ".tasktracker/docs-seed.json"

type seedCursor struct {
	Summarized map[string]bool `json:"summarized"`
}

func loadSeedCursor(root string) *seedCursor {
	c := &seedCursor{Summarized: map[string]bool{}}
	data, err := os.ReadFile(filepath.Join(root, seedCursorPath))
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, c)
	if c.Summarized == nil {
		c.Summarized = map[string]bool{}
	}
	return c
}

func (c *seedCursor) save(root string) error {
	p := filepath.Join(root, seedCursorPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return model.WriteFileAtomic(p, append(data, '\n'), 0o644)
}

// Seed performs the initial run-through over the covered tree in three
// phases: (1) every covered directory gets its STRUCTURE.md skeleton
// (deterministic, idempotent); (2) directories not yet summarized get one
// Summarizer pass each, bottom-up, bounded by opts.Budget and resumable via
// the cursor file — a dir's freshly summarized purpose is read back into the
// tree; (3) a settle pass rebuilds skeletons so parent rollups quote the
// summarized child purposes. sum may be nil (offline): skeletons are still
// written and dirs stay pending.
//
// Failures of individual directories are reported to out and do not stop the
// run; Seed returns a non-nil error at the end if any directory failed.
func Seed(ctx context.Context, root string, cfg *Config, sum Summarizer, opts SeedOptions, out io.Writer) error {
	if opts.Date == "" {
		opts.Date = time.Now().Format("2006-01-02")
	}
	tree, err := Walk(root, cfg)
	if err != nil {
		return err
	}
	cursor := loadSeedCursor(root)
	budget := opts.Budget
	var failures []string
	dirs := PostOrder(tree)

	// Phase 1: skeletons.
	skel := map[string]string{}
	for _, d := range dirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		label, err := seedSkeleton(d, opts, out)
		if err != nil {
			failures = append(failures, d.Rel)
			fmt.Fprintf(out, "%-9s %s (%v)\n", "error", d.Rel, err)
			continue
		}
		skel[d.Rel] = label
	}

	// Phase 2: summarization.
	for _, d := range dirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		label, built := skel[d.Rel]
		if !built {
			continue // skeleton failed; already recorded
		}
		switch {
		case cursor.Summarized[d.Rel]:
			fmt.Fprintf(out, "%-9s %s (already summarized)\n", label, d.Rel)
		case opts.DryRun:
			fmt.Fprintf(out, "%-9s %s (would summarize)\n", label, d.Rel)
		case sum == nil:
			fmt.Fprintf(out, "%-9s %s (pending: no summarizer)\n", label, d.Rel)
		case budget == 0 && opts.Budget > 0:
			fmt.Fprintf(out, "%-9s %s (pending: budget exhausted)\n", label, d.Rel)
		default:
			if err := summarizeOne(ctx, root, d, sum); err != nil {
				failures = append(failures, d.Rel)
				fmt.Fprintf(out, "%-9s %s (%v)\n", "failed", d.Rel, err)
				continue
			}
			budget--
			cursor.Summarized[d.Rel] = true
			if err := cursor.save(root); err != nil {
				return fmt.Errorf("save cursor: %w", err)
			}
			refreshPurpose(d)
			fmt.Fprintf(out, "%-9s %s (summarized)\n", label, d.Rel)
		}
	}

	// Phase 3: settle rollups so parents quote freshly summarized purposes.
	if !opts.DryRun {
		for _, d := range dirs {
			if _, built := skel[d.Rel]; !built {
				continue
			}
			label, err := seedSkeleton(d, opts, out)
			if err != nil {
				failures = append(failures, d.Rel)
				fmt.Fprintf(out, "%-9s %s (%v)\n", "error", d.Rel, err)
				continue
			}
			if label == "updated" {
				fmt.Fprintf(out, "%-9s %s (rollup settled)\n", label, d.Rel)
			}
		}
	}

	if len(failures) > 0 {
		sort.Strings(failures)
		return fmt.Errorf("seed failed for: %s", strings.Join(failures, ", "))
	}
	return nil
}

// refreshPurpose re-reads d's purpose line from its on-disk STRUCTURE.md
// after a summarizer pass, so parent rollups quote the fresh text.
func refreshPurpose(d *Dir) {
	data, err := os.ReadFile(filepath.Join(d.Abs, StructureFile))
	if err != nil {
		return
	}
	purpose, _, _, err := carryForward(d.Rel, data)
	if err == nil {
		d.Purpose = purpose
	}
}

// seedSkeleton builds (and, unless dry-run, writes) d's STRUCTURE.md,
// reporting whether it is new, updated, or unchanged.
func seedSkeleton(d *Dir, opts SeedOptions, out io.Writer) (string, error) {
	p := filepath.Join(d.Abs, StructureFile)
	existing, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	auto, err := Build(d, existing, model.DocMeta{Refreshed: opts.Date, Source: "seed"})
	if err != nil {
		return "", err
	}
	merged, err := model.MergeDoc(d.Rel+"/"+StructureFile, existing, []byte(auto))
	if err != nil {
		return "", err
	}
	label := "unchanged"
	switch {
	case len(existing) == 0:
		label = "new"
	case string(merged) != string(existing):
		label = "updated"
	}
	if !opts.DryRun && label != "unchanged" {
		if err := model.WriteFileAtomic(p, merged, 0o644); err != nil {
			return "", err
		}
	}
	if opts.DryRun {
		return "dry-run:" + label, nil
	}
	return label, nil
}

// summarizeOne runs one summarizer pass for d with snapshot/verify/restore:
// the pass may only annotate STRUCTURE.md (placeholders, unchanged meta) and
// create or extend AGENTS.md's auto section. Content outside the markers of
// either file must stay byte-identical. Violations are rolled back.
func summarizeOne(ctx context.Context, root string, d *Dir, sum Summarizer) error {
	structPath := filepath.Join(d.Abs, StructureFile)
	agentsPath := filepath.Join(d.Abs, AgentsFile)
	snapStruct, errStruct := os.ReadFile(structPath)
	snapAgents, errAgents := os.ReadFile(agentsPath)

	restore := func() {
		if errStruct == nil {
			_ = model.WriteFileAtomic(structPath, snapStruct, 0o644)
		}
		if errAgents == nil {
			_ = model.WriteFileAtomic(agentsPath, snapAgents, 0o644)
		} else if _, err := os.Stat(agentsPath); err == nil {
			// AGENTS.md did not exist before the pass; roll back its creation.
			_ = os.Remove(agentsPath)
		}
	}

	if err := sum.SummarizeDir(ctx, root, d); err != nil {
		restore()
		return err
	}
	if err := verifySummarized(d, structPath, agentsPath, snapStruct, errStruct, snapAgents, errAgents); err != nil {
		restore()
		return err
	}
	return nil
}

// verifySummarized checks the post-pass state: STRUCTURE.md still parses with
// meta matching the built tree and out-of-marker bytes unchanged; AGENTS.md
// (created or pre-existing) parses, keeps out-of-marker bytes identical, and
// uses markers when newly created; no pre-existing file was deleted.
func verifySummarized(d *Dir, structPath, agentsPath string, snapStruct []byte, errStruct error, snapAgents []byte, errAgents error) error {
	data, err := os.ReadFile(structPath)
	if err != nil {
		return fmt.Errorf("read STRUCTURE.md after pass: %w", err)
	}
	doc, err := model.ParseDocFile(d.Rel+"/"+StructureFile, data)
	if err != nil {
		return fmt.Errorf("STRUCTURE.md corrupt after pass: %w", err)
	}
	meta, err := model.ParseDocMeta(d.Rel+"/"+StructureFile, doc.Auto)
	if err != nil {
		return err
	}
	if meta == nil || meta.TreeHash != d.Hash {
		return fmt.Errorf("STRUCTURE.md meta missing or tree hash changed after pass")
	}
	if errStruct == nil {
		pre, err := model.ParseDocFile(d.Rel+"/"+StructureFile, snapStruct)
		if err != nil {
			return err
		}
		if pre.Prefix != doc.Prefix || pre.Suffix != doc.Suffix {
			return fmt.Errorf("STRUCTURE.md: content outside the tasktracker markers changed")
		}
	}
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		if errAgents == nil && os.IsNotExist(err) {
			return fmt.Errorf("AGENTS.md was deleted")
		}
		return nil // still absent; fine
	}
	adoc, err := model.ParseDocFile(d.Rel+"/"+AgentsFile, agents)
	if err != nil {
		return fmt.Errorf("AGENTS.md corrupt after pass: %w", err)
	}
	if errAgents == nil {
		pre, err := model.ParseDocFile(d.Rel+"/"+AgentsFile, snapAgents)
		if err != nil {
			return err
		}
		if pre.Prefix != adoc.Prefix || pre.Suffix != adoc.Suffix {
			return fmt.Errorf("AGENTS.md: content outside the tasktracker markers changed")
		}
	} else if !adoc.HasAuto {
		return fmt.Errorf("created AGENTS.md has no tasktracker marker section")
	}
	return nil
}
