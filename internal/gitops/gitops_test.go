package gitops

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// realClient returns a client over a fresh git repository with one commit on
// main, plus the client and repo dir.
func realClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	c := New(dir, filepath.Join(dir, ".lessmess"))
	ctx := context.Background()
	must := func(args ...string) {
		t.Helper()
		if _, err := c.git(ctx, dir, args...); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	must("init", "-b", "main")
	must("config", "user.email", "test@example.com")
	must("config", "user.name", "Test")
	must("commit", "--allow-empty", "-m", "init")
	return c, dir
}

func TestIsRepoAndCurrentBranch(t *testing.T) {
	c, dir := realClient(t)
	ctx := context.Background()
	if !c.IsRepo(ctx) {
		t.Fatal("IsRepo = false, want true")
	}
	if got, err := c.CurrentBranch(ctx); err != nil || got != "main" {
		t.Errorf("CurrentBranch = %q, %v; want main", got, err)
	}

	empty := New(t.TempDir(), "")
	if empty.IsRepo(ctx) {
		t.Error("IsRepo = true for a non-repo, want false")
	}
	if _, err := empty.CurrentBranch(ctx); err == nil {
		t.Error("CurrentBranch succeeded in a non-repo, want error")
	}
	_ = dir
}

func TestCreateBranch(t *testing.T) {
	c, _ := realClient(t)
	ctx := context.Background()
	if err := c.CreateBranch(ctx, "change/x", "main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := c.CreateBranch(ctx, "change/x", "main"); !errors.Is(err, ErrBranchExists) {
		t.Errorf("second CreateBranch = %v, want ErrBranchExists", err)
	}
}

func TestDirtyPaths(t *testing.T) {
	c, dir := realClient(t)
	ctx := context.Background()
	if paths, err := c.DirtyPaths(ctx); err != nil || len(paths) != 0 {
		t.Fatalf("DirtyPaths = %v, %v; want empty", paths, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := c.DirtyPaths(ctx)
	if err != nil || len(paths) != 1 || paths[0] != "a.txt" {
		t.Errorf("DirtyPaths = %v, %v; want [a.txt]", paths, err)
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	c, dir := realClient(t)
	ctx := context.Background()
	if err := c.CreateBranch(ctx, "change/x", "main"); err != nil {
		t.Fatal(err)
	}
	wt := WorktreeDir(dir, "2026-09-17-x")
	if !strings.HasSuffix(wt, filepath.Join(filepath.Base(dir)+"-worktrees", "2026-09-17-x")) {
		t.Errorf("WorktreeDir = %q, want a sibling worktrees dir", wt)
	}
	if err := c.WorktreeAdd(ctx, wt, "change/x"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if !c.HasWorktree(ctx, wt) {
		t.Fatal("HasWorktree = false after add")
	}
	if dirty, err := c.WorktreeDirty(ctx, wt); err != nil || dirty {
		t.Errorf("WorktreeDirty = %v, %v; want false", dirty, err)
	}

	// A new file makes it dirty; removal must refuse with ErrDirty.
	if err := os.WriteFile(filepath.Join(wt, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := c.WorktreeDirty(ctx, wt); err != nil || !dirty {
		t.Fatalf("WorktreeDirty = %v, %v; want true", dirty, err)
	}
	if err := c.WorktreeRemove(ctx, wt); !errors.Is(err, ErrDirty) {
		t.Errorf("remove dirty = %v, want ErrDirty", err)
	}

	// Commit inside the worktree, then removal succeeds.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = wt
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("add", "-A")
	run("-c", "user.email=t@e.com", "-c", "user.name=T", "commit", "-m", "wip")
	if err := c.WorktreeRemove(ctx, wt); err != nil {
		t.Fatalf("remove clean: %v", err)
	}
	if c.HasWorktree(ctx, wt) {
		t.Error("HasWorktree = true after remove")
	}
	if err := c.WorktreeRemove(ctx, wt); !errors.Is(err, ErrNotFound) {
		t.Errorf("remove missing = %v, want ErrNotFound", err)
	}
}

func TestWorktreeList(t *testing.T) {
	c, dir := realClient(t)
	ctx := context.Background()
	if err := c.CreateBranch(ctx, "change/y", "main"); err != nil {
		t.Fatal(err)
	}
	wt := WorktreeDir(dir, "y")
	if err := c.WorktreeAdd(ctx, wt, "change/y"); err != nil {
		t.Fatal(err)
	}
	list, err := c.WorktreeList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("WorktreeList = %+v, want main tree + one worktree", list)
	}
	byBranch := map[string]bool{}
	for _, w := range list {
		byBranch[w.Branch] = true
	}
	if !byBranch["main"] || !byBranch["change/y"] {
		t.Errorf("WorktreeList branches = %v, want main and change/y", byBranch)
	}
}

func TestPush(t *testing.T) {
	c, dir := realClient(t)
	ctx := context.Background()
	// No remote: a typed error before any network attempt.
	if err := c.Push(ctx, "main"); !errors.Is(err, ErrNoRemote) {
		t.Fatalf("Push without remote = %v, want ErrNoRemote", err)
	}

	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("remote add: %v\n%s", err, out)
	}
	if err := c.Push(ctx, "main"); err != nil {
		t.Fatalf("Push to local bare remote: %v", err)
	}
}

func TestCopyProjectConfig(t *testing.T) {
	c, dir := realClient(t)
	ctx := context.Background()
	wt := t.TempDir()
	// No opencode.json: a no-op.
	if err := c.CopyProjectConfig(ctx, wt); err != nil {
		t.Fatalf("copy without source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "opencode.json")); !os.IsNotExist(err) {
		t.Fatal("opencode.json written without a source")
	}
	src := []byte(`{"permissions":{"edit":"allow"}}`)
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.CopyProjectConfig(ctx, wt); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(wt, "opencode.json"))
	if err != nil || string(got) != string(src) {
		t.Errorf("copied config = %q, %v; want %q", got, err, src)
	}
}

// scriptedCommander returns canned output per command name, recording calls.
type scriptedCommander struct {
	fn    func(dir, name string, args []string) (string, error)
	calls [][2]string // name + joined args per call
}

func (s *scriptedCommander) Output(_ context.Context, dir, name string, args ...string) (string, error) {
	s.calls = append(s.calls, [2]string{name, strings.Join(args, " ")})
	return s.fn(dir, name, args)
}

func TestPushAndPRArgConstruction(t *testing.T) {
	sc := &scriptedCommander{fn: func(_, name string, args []string) (string, error) {
		switch {
		case name == "git" && args[0] == "remote":
			return "origin\n", nil
		case name == "gh" && args[0] == "--version":
			return "gh version 2.0.0\n", nil
		case name == "gh" && args[0] == "pr" && args[1] == "create":
			return "Creating pull request for change/x...\nhttps://github.com/o/r/pull/9\n", nil
		}
		return "", nil
	}}
	c := New("/repo", "").WithCommander(sc)
	ctx := context.Background()

	if err := c.Push(ctx, "change/x"); err != nil {
		t.Fatalf("Push: %v", err)
	}
	url, err := c.CreatePR(ctx, "change/x", "Title", "/tmp/body.md")
	if err != nil {
		t.Fatalf("CreatePR: %v", err)
	}
	if url != "https://github.com/o/r/pull/9" {
		t.Errorf("PR url = %q", url)
	}
	if err := c.CommentPR(ctx, url, "/tmp/review.md"); err != nil {
		t.Fatalf("CommentPR: %v", err)
	}

	var pushed, created, commented bool
	for _, call := range sc.calls {
		switch {
		case call[0] == "git" && strings.HasPrefix(call[1], "push -u origin change/x"):
			pushed = true
		case call[0] == "gh" && strings.HasPrefix(call[1], "pr create --head change/x --title Title --body-file /tmp/body.md"):
			created = true
		case call[0] == "gh" && strings.HasPrefix(call[1], "pr comment "+url+" --body-file /tmp/review.md"):
			commented = true
		}
	}
	if !pushed || !created || !commented {
		t.Errorf("calls = %v, want push, pr create, and pr comment", sc.calls)
	}
}

func TestGHErrors(t *testing.T) {
	sc := &scriptedCommander{fn: func(_, name string, args []string) (string, error) {
		if name == "gh" {
			return "", errors.New("exec: gh not found")
		}
		return "", nil
	}}
	c := New("/repo", "").WithCommander(sc)
	ctx := context.Background()
	if _, err := c.CreatePR(ctx, "b", "t", "/tmp/b.md"); !errors.Is(err, ErrGHUnavailable) {
		t.Errorf("CreatePR without gh = %v, want ErrGHUnavailable", err)
	}
	if err := c.CommentPR(ctx, "https://x/pull/1", "/tmp/b.md"); !errors.Is(err, ErrGHUnavailable) {
		t.Errorf("CommentPR without gh = %v, want ErrGHUnavailable", err)
	}
}

func TestPushNoRemoteNeverCallsPush(t *testing.T) {
	sc := &scriptedCommander{fn: func(_, name string, args []string) (string, error) {
		if name == "git" && args[0] == "remote" {
			return "", nil
		}
		return "", nil
	}}
	c := New("/repo", "").WithCommander(sc)
	if err := c.Push(context.Background(), "b"); !errors.Is(err, ErrNoRemote) {
		t.Fatalf("Push = %v, want ErrNoRemote", err)
	}
	for _, call := range sc.calls {
		if call[0] == "git" && strings.HasPrefix(call[1], "push") {
			t.Fatal("push attempted without a remote")
		}
	}
}
