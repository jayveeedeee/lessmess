package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lessmess/internal/gitops"
)

// reviewTimeout bounds the unattended reviewer session. The close request
// blocks on it by design (the user gated close on the PR); if the HTTP
// client gives up first, the server-side pipeline still finishes and the
// board catches up via SSE.
const reviewTimeout = 6 * time.Minute

// closeWorktreePipeline runs the gated close steps for a worktree-backed
// change, in order: clean worktree → push → PR (body from plan.md) →
// unattended reviewer session writing changes/<id>/review.md → PR comment.
// Every failure is returned typed (gitops sentinels or gitMechanicsError)
// so the handler can block the close with an actionable message while the
// change status stays untouched.
func (s *Server) closeWorktreePipeline(id string) error {
	e, ok := s.worktreeEntry(id)
	if !ok {
		return fmt.Errorf("%w: no live worktree registered for change %s", gitops.ErrNotFound, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), reviewTimeout+2*time.Minute)
	defer cancel()

	// 1. Clean gate: nothing uncommitted except the change's own metadata
	// (status flips and the reviewer's review.md live under
	// changes/<id>/ and travel with the next commit). Code dirt blocks.
	dirtyPaths, err := s.git.WorktreeDirtyPaths(ctx, e.Path)
	if err != nil {
		return &gitMechanicsError{fmt.Errorf("check worktree state: %w", err)}
	}
	var blocking []string
	for _, p := range dirtyPaths {
		if !strings.HasPrefix(p, "changes/"+id+"/") {
			blocking = append(blocking, p)
		}
	}
	if len(blocking) > 0 {
		return fmt.Errorf("%w: change %s has uncommitted files in its worktree (%s) — commit them (board Commit button) and try closing again", gitops.ErrDirty, id, strings.Join(blocking, ", "))
	}

	// 2. Push the branch (a no-op or fast-forward on a re-close).
	if err := s.git.Push(ctx, e.Branch); err != nil {
		return &gitMechanicsError{fmt.Errorf("push %s: %w", e.Branch, err)}
	}

	// 3. Open the PR with plan.md as the body. A PR left over from a
	// previous close attempt (e.g. the reviewer failed) is reused, so the
	// pipeline stays re-runnable instead of tripping "already exists".
	prURL := e.PRURL
	if prURL == "" {
		title := changeTitle(s.st, id)
		bodyFile, cleanup, err := s.prBodyFile(id, title)
		if err != nil {
			return &gitMechanicsError{err}
		}
		defer cleanup()
		prURL, err = s.git.CreatePR(ctx, e.Branch, title, bodyFile)
		if err != nil {
			return &gitMechanicsError{fmt.Errorf("create PR: %w", err)}
		}
		slog.Info("PR created", "change", id, "pr", prURL)
	} else {
		slog.Info("reusing open PR from a previous close attempt", "change", id, "pr", prURL)
	}
	if err := s.git.UpdateState(id, func(u gitops.Entry) gitops.Entry {
		u.PRURL = prURL
		u.ReviewState = gitops.ReviewPending
		return u
	}); err != nil {
		return &gitMechanicsError{fmt.Errorf("record PR: %w", err)}
	}
	slog.Info("PR created", "change", id, "pr", prURL)

	// 4. Reviewer session: unattended, bounded, worktree-scoped.
	if err := s.runReviewer(ctx, id, e); err != nil {
		_ = s.git.UpdateState(id, func(u gitops.Entry) gitops.Entry {
			u.ReviewState = gitops.ReviewFailed
			return u
		})
		return err
	}
	if err := s.git.UpdateState(id, func(u gitops.Entry) gitops.Entry {
		u.ReviewState = gitops.ReviewDone
		return u
	}); err != nil {
		return &gitMechanicsError{fmt.Errorf("record review: %w", err)}
	}

	// 5. Post the review as a PR comment.
	reviewFile := filepath.Join(e.Path, "changes", id, "review.md")
	if err := s.git.CommentPR(ctx, prURL, reviewFile); err != nil {
		// The review exists in the change dir; commenting is mechanical and
		// its failure is recorded but does not undo the close.
		slog.Warn("PR comment failed; review stays in the change dir", "change", id, "err", err)
	}
	slog.Info("close pipeline complete", "change", id, "pr", prURL)
	return nil
}

// prBodyFile writes the PR body (plan.md plus a header) to a temp file.
func (s *Server) prBodyFile(id, title string) (string, func(), error) {
	plan, err := s.st.PlanFile(id)
	if err != nil {
		return "", nil, fmt.Errorf("read plan.md for the PR body: %w", err)
	}
	f, err := os.CreateTemp("", "lessmess-pr-*.md")
	if err != nil {
		return "", nil, err
	}
	body := "PR for change `" + id + "`\n\n" +
		"Change record: `changes/" + id + "/` (included on this branch).\n\n" +
		"## Plan\n\n" + plan + "\n"
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

// runReviewer spawns, primes, awaits an unattended reviewer session working
// in the change's worktree. Unlike the docs gardener it is NOT deleted when
// it finishes: it is bound to the change, so the board's Sessions list and
// Chat lets you continue the conversation with the full context of its
// own review. The reviewer writes changes/<id>/review.md and touches
// nothing else; the reply itself is not captured (the file is the
// artifact).
func (s *Server) runReviewer(ctx context.Context, id string, e gitops.Entry) error {
	title := id + " — PR review"
	sess, err := spawnSessionWithModel(ctx, s.oc, s.st.Dir, e.Path, title, ReviewModel(s.st.Dir))
	if err != nil {
		return &gitMechanicsError{fmt.Errorf("create reviewer session: %w", err)}
	}

	prime := fmt.Sprintf(`You are the PR reviewer for change %[1]s. Work in your current directory — the change's worktree on branch %[2]s (base: %[3]s).

1. Read changes/%[1]s/plan.md — the change's authoritative scope — and the diff: run git diff %[3]s...HEAD (fall back to git diff main...HEAD, then git log -p --reverse main..HEAD if the base ref is gone).
2. Review the diff as a senior engineer: correctness, safety, tests, and whether the implementation matches the plan. You may read any file in the worktree for context.
3. Write your review to changes/%[1]s/review.md with: a Verdict line (Approve or Request changes), findings as a numbered list (severity-tagged, file:line referenced), and a short Risks section. If you find nothing, say so explicitly.

HARD RULES: write ONLY changes/%[1]s/review.md — touch no other file. Do not run any git command that changes state (no commit, push, checkout, reset, rebase, stash). Do not create or close PRs.

Reply with a one-line verdict summary.`, id, e.Branch, e.Base)
	rctx, cancel := context.WithTimeout(ctx, reviewTimeout)
	defer cancel()
	if err := s.oc.Prompt(rctx, sess.ID, prime); err != nil {
		// A session that never started its review is unbound and useless:
		// delete it (nothing unbound leaks).
		dctx, dcancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer dcancel()
		_ = s.oc.DeleteSession(dctx, sess.ID)
		return &gitMechanicsError{fmt.Errorf("prime reviewer: %w", err)}
	}
	// Bind as soon as the review is underway: finished or failed, the
	// session stays on the board for follow-up questions in Chat.
	if s.mapErr == nil {
		if err := s.sessions.add(id, SessionEntry{
			Session: sess.ID, Title: title,
			Created: time.Now().Format(time.RFC3339),
		}); err != nil {
			slog.Warn("reviewer session binding failed", "change", id, "session", sess.ID, "err", err)
		}
	}
	if err := s.oc.WaitDone(rctx, sess.ID); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return &gitMechanicsError{fmt.Errorf("reviewer timed out after %s — the session stays open on the board if you want to check on it", reviewTimeout)}
		}
		return &gitMechanicsError{fmt.Errorf("wait reviewer: %w", err)}
	}
	reviewFile := filepath.Join(e.Path, "changes", id, "review.md")
	if _, err := os.Stat(reviewFile); err != nil {
		return &gitMechanicsError{fmt.Errorf("reviewer wrote no review.md — its session is on the board if you want to nudge it")}
	}
	return nil
}
