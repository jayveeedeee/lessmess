package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lessmess/internal/gitops"
	"lessmess/internal/store"
)

// worktreeChangeRoot resolves worktree-backed changes for the store: a
// change whose docs live inside its registered worktree. The worktrees
// state file names the candidate; git (`worktree list`) must still confirm
// it, and the changes/<id> directory must exist inside it. Wired once in
// New; the store consults it on every reload for root rows without a
// main-tree directory.
func (s *Server) worktreeChangeRoot() store.ChangeRootFunc {
	if s.git == nil {
		return nil
	}
	return resolveWorktreeChange(s.git)
}

// WorktreeChangeRoot returns the worktree resolver for the repository at
// repoDir — the exported path for the CLI validate command, mirroring
// SessionDefaults.
func WorktreeChangeRoot(repoDir string) store.ChangeRootFunc {
	return resolveWorktreeChange(gitops.New(repoDir, filepath.Join(repoDir, store.StateDirName)))
}

func resolveWorktreeChange(g *gitops.Client) store.ChangeRootFunc {
	return func(id string) (string, bool) {
		st, err := g.GetState()
		if err != nil {
			slog.Warn("worktrees state unreadable; change unresolved", "change", id, "err", err)
			return "", false
		}
		e, ok := st[id]
		if !ok || e.Path == "" {
			return "", false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if !g.HasWorktree(ctx, e.Path) {
			return "", false // stale state entry; self-heals to unresolved
		}
		p := filepath.Join(e.Path, "changes", id)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, true
		}
		return "", false
	}
}

// worktreesEnabled reports whether the worktree pipeline applies for this
// request: the setting is on and the git client exists (always true outside
// setup mode).
func (s *Server) worktreesEnabled() bool {
	return s.effectiveSettings().Git.Worktrees && s.git != nil
}

// worktreeSetup is the result of the git side of a worktree-backed change.
type worktreeSetup struct {
	Branch  string
	Base    string
	Path    string
	Warning string // uncommitted main-tree files the worktree will not contain
}

// maxWarningPaths caps the dirt warning's file list.
const maxWarningPaths = 10

// gitMechanicsError marks failures of the git mechanics layer (worktree
// setup, close pipeline). They are environment problems, not store
// corruption: the wrapped message is agent-facing and maps to 502.
type gitMechanicsError struct{ err error }

func (e *gitMechanicsError) Error() string { return e.err.Error() }
func (e *gitMechanicsError) Unwrap() error { return e.err }

// setupWorktree prepares the git side of a worktree-backed change with a
// pre-minted ID: branch `change/<id>` from the configured base (or the
// current branch), a worktree for it, the project config copy, and the
// state entry. It writes no workflow files — the caller aborts on error
// before the first write, and the fresh branch (and worktree) are rolled
// back so nothing dangles.
func (s *Server) setupWorktree(ctx context.Context, id string) (worktreeSetup, error) {
	ws, err := s.setupWorktreeInner(ctx, id)
	if err != nil {
		return ws, &gitMechanicsError{err}
	}
	return ws, nil
}

func (s *Server) setupWorktreeInner(ctx context.Context, id string) (worktreeSetup, error) {
	var ws worktreeSetup
	eff := s.effectiveSettings()
	if !s.git.IsRepo(ctx) {
		return ws, errors.New("git.worktrees is enabled but the served directory is not a git repository")
	}
	base := strings.TrimSpace(eff.Git.DefaultBranch)
	if base == "" {
		b, err := s.git.CurrentBranch(ctx)
		if err != nil {
			return ws, fmt.Errorf("resolve base branch: %w", err)
		}
		base = b
	}
	ws.Base = base
	ws.Branch = "change/" + id
	if err := s.git.CreateBranch(ctx, ws.Branch, base); err != nil {
		return ws, fmt.Errorf("create branch %s from %s: %w", ws.Branch, base, err)
	}
	ws.Path = gitops.WorktreeDir(s.st.Dir, id)
	if err := s.git.WorktreeAdd(ctx, ws.Path, ws.Branch); err != nil {
		if dbErr := s.git.DeleteBranch(ctx, ws.Branch); dbErr != nil {
			slog.Warn("branch rollback after failed worktree add", "branch", ws.Branch, "err", dbErr)
		}
		return ws, fmt.Errorf("create worktree: %w", err)
	}
	if err := s.git.CopyProjectConfig(ctx, ws.Path); err != nil {
		slog.Warn("opencode.json copy into worktree failed; sessions may lack the permission allowlist", "err", err)
	}
	// Dirt check before scaffold's own writes, so the warning never lists
	// this scaffold's root-ledger row. Dirt is allowed, never refused.
	if paths, err := s.git.DirtyPaths(ctx); err != nil {
		slog.Warn("main-tree dirt check failed; warning omitted", "err", err)
	} else if len(paths) > 0 {
		shown := paths
		if len(shown) > maxWarningPaths {
			shown = append(shown[:maxWarningPaths], fmt.Sprintf("… and %d more", len(paths)-maxWarningPaths))
		}
		ws.Warning = fmt.Sprintf(
			"The main tree has uncommitted files that the new worktree will not contain (it was cut from the committed base): %s. Commit or stash them in the main tree separately if they belong to this change.",
			strings.Join(shown, ", "))
	}
	if err := s.git.UpdateState(id, func(e gitops.Entry) gitops.Entry {
		e.Branch = ws.Branch
		e.Base = ws.Base
		e.Path = ws.Path
		e.Created = time.Now().UTC().Format(time.RFC3339)
		return e
	}); err != nil {
		// Nothing must dangle: roll the worktree and the fresh branch back.
		if rmErr := s.git.WorktreeRemove(ctx, ws.Path); rmErr != nil {
			slog.Warn("worktree rollback after state failure", "path", ws.Path, "err", rmErr)
		}
		if dbErr := s.git.DeleteBranch(ctx, ws.Branch); dbErr != nil {
			slog.Warn("branch rollback after state failure", "branch", ws.Branch, "err", dbErr)
		}
		return ws, fmt.Errorf("record worktree state: %w", err)
	}
	slog.Info("worktree created", "change", id, "branch", ws.Branch, "path", ws.Path)
	return ws, nil
}

// withWorktreeRule appends the worktree stanza to a change/task prime when
// the change is worktree-backed; without one, base is returned unchanged so
// main-tree prompts stay byte-identical to the builders.
func (s *Server) withWorktreeRule(id, base string) string {
	stanza := s.worktreeStanza(id)
	if stanza == "" {
		return base
	}
	return base + "\n\n" + stanza
}

// changeSessionDir is the working directory for sessions bound to change
// id: the change's worktree when it has a live one, else the main tree.
// Settings always come from the main tree; only the session's directory
// follows the worktree.
func (s *Server) changeSessionDir(id string) string {
	if e, ok := s.worktreeEntry(id); ok {
		return e.Path
	}
	return s.st.Dir
}

// worktreeEntry returns the worktrees-state entry for a change, probing
// git so a stale entry (worktree deleted by hand) self-heals to absent.
func (s *Server) worktreeEntry(id string) (gitops.Entry, bool) {
	if s.git == nil {
		return gitops.Entry{}, false
	}
	st, err := s.git.GetState()
	if err != nil {
		return gitops.Entry{}, false
	}
	e, ok := st[id]
	if !ok || e.Path == "" {
		return gitops.Entry{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !s.git.HasWorktree(ctx, e.Path) {
		return gitops.Entry{}, false
	}
	return e, true
}

// worktreeStanza is appended to change/task primes for worktree-backed
// changes: the working-directory rule, the central root-ledger rule, and
// the push prohibition (the server pushes at close).
func (s *Server) worktreeStanza(id string) string {
	e, ok := s.worktreeEntry(id)
	if !ok {
		return ""
	}
	return fmt.Sprintf(`Worktree: this change's files — including changes/%[1]s/ — live in the worktree at %[2]s on branch %[3]s, which is your working directory for all file edits and commits. NEVER edit the root ledger (changes/ledger.md): the server maintains it centrally in the main tree, and change branches never touch it. Commit normally but never push: the server pushes this branch and opens the PR when the user closes the change.`, id, e.Path, e.Branch)
}

// worktreeViewFor builds the board header strip for one change: nil unless
// the worktrees state carries an entry for it. Probes are lazy — at most a
// state-file read plus two git calls, only for worktree-backed changes.
func (s *Server) worktreeViewFor(id string) *worktreeView {
	if s.git == nil {
		return nil
	}
	st, err := s.git.GetState()
	if err != nil || st[id].Path == "" {
		return nil
	}
	e := st[id]
	wv := &worktreeView{Branch: e.Branch, Path: e.Path, PRURL: e.PRURL, Review: e.ReviewState}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !s.git.HasWorktree(ctx, e.Path) {
		wv.State = "missing"
		return wv
	}
	if dirty, err := s.git.WorktreeDirty(ctx, e.Path); err == nil && dirty {
		wv.State = "dirty"
	} else {
		wv.State = "active"
	}
	return wv
}

// worktreeRemove handles POST /changes/{id}/worktree/remove: the explicit
// cleanup action. Refuses a dirty worktree (422) and reports a missing one
// as removable stale state; feature-off is 409.
func (s *Server) worktreeRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.worktreesEnabled() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "the worktree pipeline is disabled"})
		return
	}
	e, ok := s.worktreeEntry(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no live worktree registered for change " + id})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.git.WorktreeRemove(ctx, e.Path); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.git.DeleteStateEntry(id); err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("worktree removed", "change", id, "path", e.Path)
	writeJSON(w, http.StatusOK, map[string]string{"removed": id})
}

// createChangeRecord is the shared scaffold core for the scaffold endpoint
// and handoff spawn: with worktrees enabled it mints the ID, prepares the
// git side, and writes the change docs into the worktree (root-ledger row
// always in the main tree); with worktrees off it delegates to the legacy
// main-tree CreateChange. Returned branch and path are empty when the
// feature is off.
func (s *Server) createChangeRecord(ctx context.Context, title, prefix string) (id, branch, wtPath, warning string, err error) {
	date := time.Now().Format("2006-01-02")
	if !s.worktreesEnabled() {
		id, err = s.st.CreateChange(title, prefix, s.effectiveSettings().Git.DefaultBranch, date)
		return id, "", "", "", err
	}
	id, err = s.st.MintChangeID(date)
	if err != nil {
		return "", "", "", "", err
	}
	ws, err := s.setupWorktree(ctx, id)
	if err != nil {
		return "", "", "", "", err
	}
	if err := s.st.CreateChangeAt(id, filepath.Join(ws.Path, "changes"), title, prefix, ws.Branch, date); err != nil {
		return "", "", "", "", err
	}
	return id, ws.Branch, ws.Path, ws.Warning, nil
}
