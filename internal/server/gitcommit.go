package server

import (
	"context"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// --- read-only git status ---

// gitFileStatus is one line of `git status --porcelain`: the two-letter
// status code and the path (for renames, "old -> new" as displayed by git).
// Added/Deleted hold per-file line counts from `git diff --numstat HEAD`
// for tracked changes; empty for untracked files (and "-" for binaries).
type gitFileStatus struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Added   string `json:"added,omitempty"`
	Deleted string `json:"deleted,omitempty"`
}

// gitRepoStatus reports the repository's uncommitted state. Repo is false
// when dir is not inside a git work tree (or git is unavailable); the UI
// hides commit affordances in that case. Summary holds the one-line
// `git diff --shortstat HEAD` output (tracked changes only).
type gitRepoStatus struct {
	Repo    bool            `json:"repo"`
	Changes []gitFileStatus `json:"changes"`
	Summary string          `json:"summary"`
}

// runGit executes a read-only git command with a short timeout so a wedged
// git cannot hang a request.
func runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	return string(out), err
}

// gitStatus inspects dir's git state. It is strictly read-only: rev-parse,
// status, and diff only. Any rev-parse failure (not a repo, no git binary)
// degrades to Repo:false rather than an error.
func gitStatus(dir string) gitRepoStatus {
	st := gitRepoStatus{Changes: []gitFileStatus{}}
	if _, err := runGit(dir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return st
	}
	st.Repo = true

	porcelain, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		slog.Warn("git status failed", "dir", dir, "err", err)
		return st
	}
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 4 { // need XY + space + at least one path char
			continue
		}
		code, path := line[:2], strings.TrimSpace(line[3:])
		// git quotes paths containing special characters.
		path = strings.Trim(path, `"`)
		st.Changes = append(st.Changes, gitFileStatus{Code: code, Path: path})
	}

	// numstat covers staged+unstaged tracked changes, merged into the
	// porcelain rows by path. On a repo with no commits yet HEAD does not
	// resolve; fall back to the staged diff (against the empty tree).
	numstat, err := runGit(dir, "diff", "--numstat", "HEAD")
	if err != nil {
		numstat, err = runGit(dir, "diff", "--numstat", "--cached")
	}
	if err == nil {
		counts := map[string][2]string{}
		for _, line := range strings.Split(numstat, "\n") {
			f := strings.SplitN(line, "\t", 3) // added<TAB>deleted<TAB>path
			if len(f) == 3 {
				counts[f[2]] = [2]string{f[0], f[1]}
			}
		}
		for i, c := range st.Changes {
			if n, ok := counts[c.Path]; ok {
				st.Changes[i].Added, st.Changes[i].Deleted = n[0], n[1]
			}
		}
	}

	shortstat, err := runGit(dir, "diff", "--shortstat", "HEAD")
	if err != nil {
		shortstat, err = runGit(dir, "diff", "--shortstat", "--cached")
	}
	if err == nil {
		st.Summary = strings.TrimSpace(shortstat)
	}
	return st
}

// gitBranches lists local branch names (read-only), sorted. Fail-open to
// empty: the options endpoint only suggests them.
func gitBranches(dir string) []string {
	out, err := runGit(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		if b := strings.TrimSpace(line); b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

// gitStatusAPI handles GET /api/git/status: always 200, degrading to
// repo:false when git state is unavailable.
func (s *Server) gitStatusAPI(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, gitStatus(s.st.Dir))
}

// --- repo-wide commit ---

// repoCommitPrompt builds the message for a repo-wide commit session: like
// commitPrompt but not tied to any change record.
func repoCommitPrompt() string {
	return `Create a git commit for this repository's current uncommitted changes.

1. Run git status and git diff (staged and unstaged) to review what is uncommitted.
2. Write a commit message following good practice: a concise summary line, then a body explaining the what and why. The change records under changes/ (if any relate to the diff) are context, not necessarily the subject.
3. Stage everything relevant with git add -A (this respects .gitignore) and create the commit with that message.
4. NEVER push, amend, rebase, reset, or switch branches. Commit only.

Report the resulting commit hash and summary when done.`
}

// commitAll handles POST /api/git/commit: re-check that the tree is dirty,
// then create + prime a repo-wide commit session mapped to the unassigned
// Discussions bucket.
func (s *Server) commitAll(w http.ResponseWriter, r *http.Request) {
	st := gitStatus(s.st.Dir)
	if !st.Repo {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "not a git repository"})
		return
	}
	if len(st.Changes) == 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "nothing to commit"})
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	sess, err := s.spawnSession(ctx, "repo — git commit")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: sess.Title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.addUnassigned(entry); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	if err := s.oc.Prompt(ctx, sess.ID, s.promptWith(repoCommitPrompt(), "repoCommit")); err != nil {
		slog.Warn("repo commit prime failed", "session", sess.ID, "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"session": sess.ID, "error": "session created, but priming failed: " + err.Error()})
		return
	}
	slog.Info("repo commit session created", "session", sess.ID)
	writeJSON(w, http.StatusCreated, map[string]string{"session": sess.ID})
}
