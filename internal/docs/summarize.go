package docs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lessmess/internal/opencode"
)

// Summarizer fills in a directory's placeholder purposes/blurbs in
// STRUCTURE.md and writes initial AGENTS.md learnings, editing the files
// directly (agent-native: the model works in the repo, the caller verifies
// the result). Seed calls it once per covered directory.
type Summarizer interface {
	SummarizeDir(ctx context.Context, root string, d *Dir) error
}

// SessionClient is the subset of the opencode service client the seed
// summarizer needs.
type SessionClient interface {
	CreateSession(ctx context.Context, title, directory string) (*opencode.Session, error)
	CreateSessionWith(ctx context.Context, title, directory, agent string, model *opencode.ModelRef) (*opencode.Session, error)
	Prompt(ctx context.Context, id, text string) error
	WaitDone(ctx context.Context, id string) error
	DeleteSession(ctx context.Context, id string) error
}

// OpenCodeSummarizer runs one unattended opencode session per directory.
type OpenCodeSummarizer struct {
	c     SessionClient
	wait  time.Duration
	agent string
	model string
}

// NewOpenCodeSummarizer wraps a service client with the service-default
// agent/model. wait bounds each session's runtime; 0 uses a 10-minute
// default.
func NewOpenCodeSummarizer(c SessionClient, wait time.Duration) *OpenCodeSummarizer {
	return NewOpenCodeSummarizerWith(c, wait, "", "")
}

// NewOpenCodeSummarizerWith is NewOpenCodeSummarizer carrying the
// configured default agent/model ("" = service default; model is a single
// "providerID/id" string). Seed sessions honor the same session defaults
// as every other lessmess-spawned session.
func NewOpenCodeSummarizerWith(c SessionClient, wait time.Duration, agent, model string) *OpenCodeSummarizer {
	if wait <= 0 {
		wait = 10 * time.Minute
	}
	return &OpenCodeSummarizer{c: c, wait: wait, agent: agent, model: model}
}

// modelRef parses the "providerID/id" setting; nil when unset, incomplete
// refs degrade to the service default (CreateSessionWith omits them).
func (s *OpenCodeSummarizer) modelRef() *opencode.ModelRef {
	if s.model == "" {
		return nil
	}
	prov, id, _ := strings.Cut(s.model, "/")
	return &opencode.ModelRef{ID: id, ProviderID: prov}
}

// SummarizeDir creates, primes, and awaits one seed session for d.
func (s *OpenCodeSummarizer) SummarizeDir(ctx context.Context, root string, d *Dir) error {
	sess, err := s.c.CreateSessionWith(ctx, "seed docs: "+d.Rel, root, s.agent, s.modelRef())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = s.c.DeleteSession(ctx, sess.ID)
	}()
	if err := s.c.Prompt(ctx, sess.ID, SeedPrompt(d)); err != nil {
		return fmt.Errorf("prompt: %w", err)
	}
	wctx, cancel := context.WithTimeout(ctx, s.wait)
	defer cancel()
	if err := s.c.WaitDone(wctx, sess.ID); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	return nil
}

// SeedPrompt builds the prompt that primes a seed session for d. The
// STRUCTURE.md skeleton (with placeholders) is already on disk; the session
// only annotates it and writes AGENTS.md learnings.
func SeedPrompt(d *Dir) string {
	var entries strings.Builder
	for _, s := range d.Subdirs {
		fmt.Fprintf(&entries, "- %s/ (covered subdirectory — its purpose is already quoted from its own STRUCTURE.md; do not change its row)\n", s)
	}
	for _, f := range d.Files {
		fmt.Fprintf(&entries, "- %s\n", f)
	}
	return fmt.Sprintf(`You are seeding agent-facing docs for exactly one directory of this repository.

Directory: %[1]s (relative to the repository root you are working in)

Its STRUCTURE.md has just been generated with em dash placeholders (—) for the directory purpose line and any entry purposes that are not yet known. Entries:
%[2]s
Do exactly this, and touch ONLY %[1]s/STRUCTURE.md and %[1]s/AGENTS.md:

1. Skim the directory's files as needed (stay within the directory and its covered children's STRUCTURE.md files).
2. In STRUCTURE.md, between the `+"`<!-- tasktracker:begin -->`"+` and `+"`<!-- tasktracker:end -->`"+` markers:
   - Replace the — purpose line with ONE concise sentence describing the directory's role.
   - Replace every remaining — file-entry purpose with a concise phrase (a few words, no trailing period).
   - Do NOT add, remove, reorder, or rename entries. Do NOT touch the tasktracker-meta line, covered-subdirectory rows, the heading, or the table structure. Do not use the characters | or newlines inside table cells.
3. In AGENTS.md (create it if needed), add or extend ONE `+"`<!-- tasktracker:begin -->`"+`/`+"`<!-- tasktracker:end -->`"+` auto section with 2–5 brief learnings for an agent working in this directory (purpose, key types/files, conventions, gotchas). Phrase each learning as a current-state fact about how the code works — no change narration, no provenance prefixes — and keep the section minimal.
   HARD RULES:
   - The file must end up with EXACTLY ONE begin marker and EXACTLY ONE end marker. Never write these marker strings anywhere else in the file — no prose mentions, no examples, no duplicates.
   - If AGENTS.md already exists, your ONLY edit is to append the auto section at the very END of the file (or extend the existing one in place). Do not re-read, reorganize, reformat, annotate, or "improve" any pre-existing content, however tempting — preserve it byte for byte.
4. Do not modify any other file. Do not run any git commands.

Reply with a one-line summary when done.`, d.Rel, entries.String())
}
