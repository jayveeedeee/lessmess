// Package gitops executes the git and GitHub CLI mechanics behind the
// worktree-per-change pipeline: branch creation, worktree add/list/remove,
// dirty checks, push, and PR create/comment. All other git awareness in the
// application stays read-only (server/gitcommit.go) and all commits happen
// inside primed opencode sessions — this package is the one sanctioned place
// for mutating git execution in Go.
//
// Every method talks to the real git binary in the served repository (or the
// given worktree path) through an injectable Commander so tests can use real
// temp repositories or scripted fakes. Errors wrap stderr for actionable
// messages; sentinel errors (ErrBranchExists, ErrNoRemote, ErrGHUnavailable,
// ErrDirty) let callers branch on the failure mode.
package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Sentinel errors callers map to distinct API responses.
var (
	// ErrBranchExists means the change branch already exists.
	ErrBranchExists = errors.New("branch already exists")
	// ErrNoRemote means the repository has no "origin" remote to push to.
	ErrNoRemote = errors.New("no remote configured")
	// ErrGHUnavailable means the gh CLI is missing or not runnable.
	ErrGHUnavailable = errors.New("gh CLI unavailable")
	// ErrDirty means the worktree has uncommitted changes.
	ErrDirty = errors.New("worktree has uncommitted changes")
	// ErrNotFound means the requested worktree is not registered in git.
	ErrNotFound = errors.New("worktree not found")
)

// Commander executes one command inside dir and returns trimmed stdout.
// Stderr is folded into the error. Production uses os/exec; tests substitute
// scripted fakes or point the client at real temp repositories.
type Commander interface {
	Output(ctx context.Context, dir, name string, args ...string) (string, error)
}

// execCommander is the production Commander.
type execCommander struct{}

func (execCommander) Output(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return string(out), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return string(out), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return string(out), nil
}

// defaultCommandTimeout bounds each command when the caller's context has no
// deadline; push and gh may legitimately take longer than local operations.
const defaultCommandTimeout = 2 * time.Minute

// ExecCommander returns the production commander (real os/exec). Exposed
// for tests that wrap it — e.g. scripting gh while passing git through.
func ExecCommander() Commander { return execCommander{} }

// Client runs git/gh operations for one served repository.
type Client struct {
	// Dir is the served repository root (the main tree).
	Dir string
	// StateDir holds the worktrees state file (normally the repository's
	// .lessmess directory); empty disables state persistence.
	StateDir string
	cmd      Commander
}

// New returns a client for the repository at dir with its tooling state in
// stateDir (pass the repository's .lessmess path).
func New(dir, stateDir string) *Client {
	return &Client{Dir: dir, StateDir: stateDir, cmd: execCommander{}}
}

// WithCommander replaces the command seam (tests).
func (c *Client) WithCommander(cmd Commander) *Client {
	c.cmd = cmd
	return c
}

// run executes one command with a default timeout when ctx has none.
func (c *Client) run(ctx context.Context, dir, name string, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultCommandTimeout)
		defer cancel()
	}
	return c.cmd.Output(ctx, dir, name, args...)
}

// git runs a git command scoped to dir.
func (c *Client) git(ctx context.Context, dir string, args ...string) (string, error) {
	return c.run(ctx, dir, "git", args...)
}

// IsRepo reports whether dir is inside a git work tree.
func (c *Client) IsRepo(ctx context.Context) bool {
	out, err := c.git(ctx, c.Dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// CurrentBranch returns the checked-out branch of the main tree.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	out, err := c.git(ctx, c.Dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
	if branch == "" || branch == "HEAD" {
		return "", errors.New("detached HEAD: no current branch to cut from")
	}
	return branch, nil
}

// DirtyPaths lists the main tree's uncommitted paths (status --porcelain),
// quoted paths unquoted the same way the status preview unquotes them.
func (c *Client) DirtyPaths(ctx context.Context) ([]string, error) {
	out, err := c.git(ctx, c.Dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		p = strings.Trim(p, `"`)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// CreateBranch creates branch name at base. It fails with ErrBranchExists
// when the branch already exists, so a scaffold retry never moves an existing
// branch.
func (c *Client) CreateBranch(ctx context.Context, name, base string) error {
	if _, err := c.git(ctx, c.Dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name); err == nil {
		return ErrBranchExists
	}
	_, err := c.git(ctx, c.Dir, "branch", name, base)
	return err
}

// WorktreeDir computes the worktree path for change id: a sibling of the
// repository root, outside the served tree so it never pollutes status,
// docs walks, or store globs.
func WorktreeDir(repoDir, id string) string {
	parent := filepath.Dir(repoDir)
	return filepath.Join(parent, filepath.Base(repoDir)+"-worktrees", id)
}

// WorktreeAdd registers path as a worktree checking out the existing branch.
func (c *Client) WorktreeAdd(ctx context.Context, path, branch string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err := c.git(ctx, c.Dir, "worktree", "add", path, branch)
	return err
}

// Worktree is one entry of `git worktree list --porcelain`.
type Worktree struct {
	Path   string
	Branch string // empty for detached or bare entries
}

// WorktreeList parses `git worktree list --porcelain`.
func (c *Client) WorktreeList(ctx context.Context) ([]Worktree, error) {
	out, err := c.git(ctx, c.Dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []Worktree
	var cur *Worktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			list = append(list, Worktree{})
			cur = &list[len(list)-1]
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			if cur != nil {
				cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
			}
		}
	}
	return list, nil
}

// HasWorktree reports whether path is a registered worktree. Comparison is
// symlink-aware: git reports resolved paths, while callers may pass paths
// through unresolved temporary or symlinked directories.
func (c *Client) HasWorktree(ctx context.Context, path string) bool {
	list, err := c.WorktreeList(ctx)
	if err != nil {
		return false
	}
	for _, w := range list {
		if samePath(w.Path, path) {
			return true
		}
	}
	return false
}

// samePath compares two paths after resolving symlinks where possible.
func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return a == b
}

// WorktreeDirty reports whether the worktree at path has uncommitted changes.
func (c *Client) WorktreeDirty(ctx context.Context, path string) (bool, error) {
	paths, err := c.WorktreeDirtyPaths(ctx, path)
	if err != nil {
		return false, err
	}
	return len(paths) > 0, nil
}

// WorktreeDirtyPaths lists the worktree's uncommitted paths (status
// --porcelain), so callers can ignore workflow metadata under the change's
// own directory.
func (c *Client) WorktreeDirtyPaths(ctx context.Context, path string) ([]string, error) {
	out, err := c.git(ctx, path, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		p = strings.Trim(p, `"`)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// WorktreeRemove removes a clean worktree. A path that is not a registered
// worktree maps to ErrNotFound; a dirty one maps to ErrDirty without
// touching git's remove machinery.
func (c *Client) WorktreeRemove(ctx context.Context, path string) error {
	if !c.HasWorktree(ctx, path) {
		return fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	dirty, err := c.WorktreeDirty(ctx, path)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("%w: commit the changes first", ErrDirty)
	}
	_, err = c.git(ctx, c.Dir, "worktree", "remove", path)
	return err
}

// DeleteBranch deletes a branch that carries no unique commits (`git branch
// -d` refuses otherwise) — the rollback path for a failed worktree setup.
func (c *Client) DeleteBranch(ctx context.Context, name string) error {
	_, err := c.git(ctx, c.Dir, "branch", "-d", name)
	return err
}

// PruneWorktrees drops worktree registrations whose directories vanished.
func (c *Client) PruneWorktrees(ctx context.Context) error {
	_, err := c.git(ctx, c.Dir, "worktree", "prune")
	return err
}

// Push pushes branch to origin with upstream tracking. ErrNoRemote is
// returned before any network attempt when origin is not configured.
func (c *Client) Push(ctx context.Context, branch string) error {
	out, err := c.git(ctx, c.Dir, "remote")
	if err != nil {
		return err
	}
	if !hasField(strings.Fields(out), "origin") {
		return fmt.Errorf("%w: add an origin remote to enable PRs", ErrNoRemote)
	}
	_, err = c.git(ctx, c.Dir, "push", "-u", "origin", branch)
	return err
}

// CreatePR opens a PR head → the remote's default branch with the given
// title and a --body-file, returning the PR URL gh prints. The body must
// already exist on disk; the caller owns building it from plan.md.
func (c *Client) CreatePR(ctx context.Context, branch, title, bodyFile string) (string, error) {
	if _, err := c.run(ctx, c.Dir, "gh", "--version"); err != nil {
		return "", fmt.Errorf("%w: %v", ErrGHUnavailable, err)
	}
	out, err := c.run(ctx, c.Dir, "gh", "pr", "create",
		"--head", branch, "--title", title, "--body-file", bodyFile)
	if err != nil {
		return "", err
	}
	return prURLFromOutput(out), nil
}

// CommentPR posts bodyFile as a comment on the PR at prURL.
func (c *Client) CommentPR(ctx context.Context, prURL, bodyFile string) error {
	if _, err := c.run(ctx, c.Dir, "gh", "--version"); err != nil {
		return fmt.Errorf("%w: %v", ErrGHUnavailable, err)
	}
	_, err := c.run(ctx, c.Dir, "gh", "pr", "comment", prURL, "--body-file", bodyFile)
	return err
}

// CopyProjectConfig copies the repository's opencode.json into the worktree
// root when one exists, so opencode project discovery sees the same agent
// permission allowlist for worktree-scoped sessions. A missing source file is
// not an error.
func (c *Client) CopyProjectConfig(ctx context.Context, worktreePath string) error {
	src := filepath.Join(c.Dir, "opencode.json")
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(worktreePath, "opencode.json"), data, 0o644)
}

// hasField reports whether needle is one of the whitespace-split fields.
func hasField(fields []string, needle string) bool {
	for _, f := range fields {
		if f == needle {
			return true
		}
	}
	return false
}

// prURLFromOutput extracts the https URL from gh pr create output (the last
// line is the new PR's URL; other lines may carry progress hints).
func prURLFromOutput(out string) string {
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return strings.TrimSpace(out)
}
