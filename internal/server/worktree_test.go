package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/gitops"
	"lessmess/internal/model"
	"lessmess/internal/opencode"
)

// gitFixtureServer builds a fixture server whose served directory is a real
// git repository, with lessmess.json enabling worktrees when asked. The
// sibling worktrees directory the scaffold creates is cleaned up on test
// end, along with the branches it carried.
func gitFixtureServer(t *testing.T, worktrees bool, commitAll bool) *Server {
	t.Helper()
	st, dir := fixtureStore(t)
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	mustGit("init", "-b", "main")
	mustGit("config", "user.email", "test@example.com")
	mustGit("config", "user.name", "Test")
	if worktrees {
		if err := os.WriteFile(filepath.Join(dir, "lessmess.json"), []byte(`{"git":{"worktrees":true}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if commitAll {
		// Tooling state is gitignored, like any real repository using
		// lessmess; everything committed here leaves the tree clean.
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".lessmess/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGit("add", "-A")
		mustGit("commit", "-m", "init")
	}
	s := New(st)
	t.Cleanup(func() {
		wtRoot := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-worktrees")
		if err := os.RemoveAll(wtRoot); err != nil {
			t.Errorf("cleanup worktrees dir: %v", err)
		}
		cmd := exec.Command("git", "-C", dir, "worktree", "prune")
		cmd.Dir = dir
		_ = cmd.Run()
		if out, err := exec.Command("git", "-C", dir, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output(); err == nil {
			for _, b := range strings.Fields(string(out)) {
				if b != "main" {
					rm := exec.Command("git", "-C", dir, "branch", "-D", b)
					rm.Dir = dir
					_ = rm.Run()
				}
			}
		}
		s.Close()
	})
	return s
}

func postScaffold(t *testing.T, s *Server, body string) map[string]string {
	t.Helper()
	w := do(t, s.Handler(), "POST", "/changes/scaffold", body)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestScaffoldWorktreeEnabled(t *testing.T) {
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_sc", Title: "disc", Created: "x"}); err != nil {
		t.Fatal(err)
	}

	resp := postScaffold(t, s, `{"title":"Worktree change","prefix":"WTS","session":"ses_sc"}`)
	id := resp["change"]
	if id == "" || resp["branch"] != "change/"+id || resp["worktree"] == "" {
		t.Fatalf("resp = %v", resp)
	}
	if resp["warning"] != "" {
		t.Errorf("clean tree produced warning %q", resp["warning"])
	}
	if _, ok := resp["worktree-note"]; !ok {
		t.Error("worktree-note missing: the planning session needs the worktree path")
	}

	// The index carries the entry with its branch; docs live in the worktree.
	if _, err := os.Stat(filepath.Join(resp["worktree"], "changes", id, "plan.md")); err != nil {
		t.Errorf("worktree plan.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.st.Dir, "changes", id)); !os.IsNotExist(err) {
		t.Error("change dir materialized in the main tree")
	}
	e := s.st.Entry(id)
	if e == nil {
		t.Fatal("index entry missing")
	}
	if e.Branch != "change/"+id {
		t.Errorf("index branch = %q", e.Branch)
	}

	// The store resolves the change through its worktree and validates clean.
	c, err := s.st.Change(id)
	if err != nil {
		t.Fatalf("resolved change: %v", err)
	}
	if !strings.HasPrefix(c.Dir, resp["worktree"]) {
		t.Errorf("Change.Dir = %q, want inside %q", c.Dir, resp["worktree"])
	}
	if v := s.st.Validate(); len(v) != 0 {
		t.Fatalf("violations after worktree scaffold: %v", v)
	}

	// State recorded for close and resolution.
	gst, err := s.git.GetState()
	if err != nil {
		t.Fatal(err)
	}
	we, ok := gst[id]
	if !ok || we.Branch != resp["branch"] || we.Path != resp["worktree"] {
		t.Errorf("worktrees state = %+v", we)
	}

	// Mapping moved.
	if len(s.sessions.listUnassigned()) != 0 {
		t.Error("bucket not emptied")
	}
	if got := s.sessions.list(id); len(got) != 1 || got[0].Session != "ses_sc" {
		t.Errorf("change sessions = %+v", got)
	}
}

func TestScaffoldWorktreeDirtyWarning(t *testing.T) {
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_sc", Title: "disc", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	// An untracked file after the clean commit: dirt is allowed but must
	// be warned about.
	if err := os.WriteFile(filepath.Join(s.st.Dir, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := postScaffold(t, s, `{"title":"Dirty","prefix":"DRT","session":"ses_sc"}`)
	if resp["branch"] == "" || resp["worktree"] == "" {
		t.Fatalf("dirt must not block the scaffold: %v", resp)
	}
	if !strings.Contains(resp["warning"], "scratch.txt") || !strings.Contains(resp["warning"], "uncommitted") {
		t.Errorf("warning = %q, want it to name uncommitted main-tree files", resp["warning"])
	}
}

func TestScaffoldWorktreeGitFailure(t *testing.T) {
	// Worktrees enabled but the served directory is not a repository.
	s := gitFixtureServer(t, true, false)
	s.SetOpencode(nil)
	// Remove the git repo the fixture made? It was never created with
	// commitAll=false — init still ran, so drop the .git dir to force the
	// not-a-repo path.
	if err := os.RemoveAll(filepath.Join(s.st.Dir, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_sc", Title: "disc", Created: "x"}); err != nil {
		t.Fatal(err)
	}

	w := do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"No repo","prefix":"NRP","session":"ses_sc"}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("code = %d body = %s, want 502", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "not a git repository") {
		t.Errorf("body = %s, want the not-a-repo explanation", w.Body)
	}
	// Nothing scaffolded: no index entry, no main-tree dir, no state.
	idx, _ := s.st.Index()
	if len(idx.Changes) != 1 {
		t.Errorf("index = %d entries, want only the fixture entry", len(idx.Changes))
	}
	if st, _ := s.git.GetState(); len(st) != 0 {
		t.Errorf("state = %v, want empty", st)
	}
}

func TestScaffoldWorktreeDisabled(t *testing.T) {
	// Git repo present but the setting off: the legacy path must not run
	// any git operations (no branch, no worktree dir, no state).
	s := gitFixtureServer(t, false, true)
	s.SetOpencode(nil)
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_sc", Title: "disc", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	resp := postScaffold(t, s, `{"title":"Legacy","prefix":"LEG","session":"ses_sc"}`)
	id := resp["change"]
	if resp["branch"] != "" || resp["worktree"] != "" || resp["warning"] != "" {
		t.Fatalf("resp = %v, want no worktree fields", resp)
	}
	if _, err := os.Stat(filepath.Join(s.st.Dir, "changes", id, "plan.md")); err != nil {
		t.Errorf("legacy scaffold must write the main tree: %v", err)
	}
	if st, _ := s.git.GetState(); len(st) != 0 {
		t.Errorf("state = %v, want empty", st)
	}
	out, err := exec.Command("git", "-C", s.st.Dir, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	if err != nil || strings.TrimSpace(string(out)) != "main" {
		t.Errorf("branches = %q, %v; want only main", out, err)
	}
}

// createDirectory digs the requested session directory out of one captured
// create body.
func createDirectory(t *testing.T, body map[string]any) string {
	t.Helper()
	loc, ok := body["location"].(map[string]any)
	if !ok {
		t.Fatal("create body has no location")
	}
	d, _ := loc["directory"].(string)
	return d
}

func TestChangeSessionsSpawnIntoWorktree(t *testing.T) {
	cap := &ocCapture{}
	s := gitFixtureServer(t, true, true)
	fake := httptest.NewServer(cap.handler())
	t.Cleanup(fake.Close)
	s.SetOpencode(opencode.New(fake.URL, "pw"))

	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_sc", Title: "disc", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	resp := postScaffold(t, s, `{"title":"Worktree change","prefix":"WTS","session":"ses_sc"}`)
	id := resp["change"]
	wt := resp["worktree"]

	// A board-created change session spawns inside the worktree and its
	// prime carries the worktree rule.
	if w := do(t, s.Handler(), "POST", "/changes/"+id+"/sessions", `{}`); w.Code != 201 {
		t.Fatalf("create session: %d %s", w.Code, w.Body)
	}
	if len(cap.creates) == 0 || createDirectory(t, cap.creates[0]) != wt {
		t.Errorf("change session create = %v, want directory %q", cap.creates, wt)
	}
	if len(cap.prompts) != 1 || !strings.Contains(cap.prompts[0], "NEVER edit workflow state files by hand") {
		t.Errorf("prime = %.120q, want the worktree stanza", cap.prompts)
	}

	// The change's commit session also runs in the worktree.
	if w := do(t, s.Handler(), "POST", "/changes/"+id+"/commit", ``); w.Code != 201 {
		t.Fatalf("commit session: %d %s", w.Code, w.Body)
	}
	if len(cap.creates) < 2 || createDirectory(t, cap.creates[len(cap.creates)-1]) != wt {
		t.Errorf("commit session create = %v, want directory %q", cap.creates, wt)
	}
}

func TestChangeSessionMainTreeWithoutWorktree(t *testing.T) {
	cap := &ocCapture{}
	s := gitFixtureServer(t, false, true)
	fake := httptest.NewServer(cap.handler())
	t.Cleanup(fake.Close)
	s.SetOpencode(opencode.New(fake.URL, "pw"))

	// Legacy scaffold: docs in the main tree, sessions there too.
	resp := postScaffold(t, s, `{"title":"Legacy","prefix":"LEG","session":"ses_sc"}`)
	id := resp["change"]
	if w := do(t, s.Handler(), "POST", "/changes/"+id+"/sessions", `{}`); w.Code != 201 {
		t.Fatalf("create session: %d %s", w.Code, w.Body)
	}
	if len(cap.creates) != 1 || createDirectory(t, cap.creates[0]) != s.st.Dir {
		t.Errorf("create = %v, want the main tree", cap.creates)
	}
	if len(cap.prompts) == 1 && strings.Contains(cap.prompts[0], "NEVER edit workflow state files by hand") {
		t.Error("main-tree prompt must not carry the worktree stanza")
	}
}

// ghFake passes git through to the real binary while scripting gh. It
// records every call so tests can assert the push/PR/comment sequence.
type ghFake struct {
	real  gitops.Commander
	gh    func(args []string) (string, error)
	calls [][2]string
}

func (f *ghFake) Output(ctx context.Context, dir, name string, args ...string) (string, error) {
	f.calls = append(f.calls, [2]string{name, strings.Join(args, " ")})
	if name == "gh" {
		return f.gh(args)
	}
	return f.real.Output(ctx, dir, name, args...)
}

// commitWorktreeFile commits one file inside the worktree so the clean gate
// passes with it present.
func commitWorktreeFile(t *testing.T, wt, rel, content string) {
	t.Helper()
	p := filepath.Join(wt, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = wt
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("add", rel)
	run("-c", "user.email=t@e.com", "-c", "user.name=T", "commit", "-m", rel)
}

// commitWorktreeAll commits everything in the worktree (the scaffold's
// own changes/<id> docs are untracked until the agent's first commit).
func commitWorktreeAll(t *testing.T, wt string) {
	t.Helper()
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
}

// scaffoldWorktreeChange scaffolds one worktree-backed change and returns
// its id and worktree path.
func scaffoldWorktreeChange(t *testing.T, s *Server) (string, string) {
	t.Helper()
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_sc", Title: "disc", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	resp := postScaffold(t, s, `{"title":"Worktree change","prefix":"WTS","session":"ses_sc"}`)
	if resp["worktree"] == "" {
		t.Fatalf("resp = %v, want a worktree", resp)
	}
	return resp["change"], resp["worktree"]
}

// addBareRemote creates a local bare remote and wires it as origin, so the
// real push works offline.
func addBareRemote(t *testing.T, dir string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "--bare", remote)
	run("remote", "add", "origin", remote)
}

func TestClosePipelineFullSuccess(t *testing.T) {
	cap := &ocCapture{}
	s := gitFixtureServer(t, true, true)
	fake := httptest.NewServer(cap.handler())
	t.Cleanup(fake.Close)
	s.SetOpencode(opencode.New(fake.URL, "pw"))
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)
	addBareRemote(t, s.st.Dir)
	commitWorktreeFile(t, wt, "changes/"+id+"/review.md", "## Verdict\nApprove")

	ghf := &ghFake{real: gitops.ExecCommander(), gh: func(args []string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
			return "https://github.com/o/r/pull/5\n", nil
		}
		return "", nil
	}}
	s.git = s.git.WithCommander(ghf)

	if w := do(t, s.Handler(), "POST", "/changes/"+id+"/close", ``); w.Code != 200 {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	if c, err := s.st.Change(id); err != nil || c.Overall() != model.OverallDone {
		t.Fatalf("status after close = %v, %v; want Done", c, err)
	}
	st, _ := s.git.GetState()
	e := st[id]
	if e.PRURL != "https://github.com/o/r/pull/5" || e.ReviewState != gitops.ReviewDone {
		t.Errorf("state = %+v, want PR URL and review done", e)
	}
	var pushed, commented bool
	for _, call := range ghf.calls {
		if call[0] == "git" && strings.HasPrefix(call[1], "push -u origin change/"+id) {
			pushed = true
		}
		if call[0] == "gh" && strings.HasPrefix(call[1], "pr comment https://github.com/o/r/pull/5") {
			commented = true
		}
	}
	if !pushed || !commented {
		t.Errorf("calls = %v, want push and comment", ghf.calls)
	}
	// The reviewer session ran inside the worktree.
	if len(cap.creates) == 0 || createDirectory(t, cap.creates[len(cap.creates)-1]) != wt {
		t.Errorf("reviewer create = %v, want directory %q", cap.creates, wt)
	}
	// Reopen reattaches: the change still resolves.
	if w := do(t, s.Handler(), "POST", "/changes/"+id+"/reopen", ``); w.Code != 200 {
		t.Fatalf("reopen: %d %s", w.Code, w.Body)
	}
	if _, err := s.st.Change(id); err != nil {
		t.Fatalf("resolve after reopen: %v", err)
	}
}

func TestCloseGateDirtyWorktree(t *testing.T) {
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)
	if err := os.WriteFile(filepath.Join(wt, "loose.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := do(t, s.Handler(), "POST", "/changes/"+id+"/close", ``)
	if w.Code != 422 {
		t.Fatalf("code = %d body = %s, want 422", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "uncommitted") {
		t.Errorf("body = %s, want commit guidance", w.Body)
	}
	if c, err := s.st.Change(id); err == nil && c.Overall() == model.OverallDone {
		t.Fatal("dirty worktree must not reach Done")
	}
}

func TestCloseGateNoRemote(t *testing.T) {
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)

	w := do(t, s.Handler(), "POST", "/changes/"+id+"/close", ``)
	if w.Code != 502 {
		t.Fatalf("code = %d body = %s, want 502", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "no remote") {
		t.Errorf("body = %s, want the no-remote message", w.Body)
	}
	if c, err := s.st.Change(id); err == nil && c.Overall() == model.OverallDone {
		t.Fatal("failed push must not reach Done")
	}
}

func TestCloseGateReviewerMustWriteReview(t *testing.T) {
	cap := &ocCapture{}
	s := gitFixtureServer(t, true, true)
	fake := httptest.NewServer(cap.handler())
	t.Cleanup(fake.Close)
	s.SetOpencode(opencode.New(fake.URL, "pw"))
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)
	addBareRemote(t, s.st.Dir)

	ghf := &ghFake{real: gitops.ExecCommander(), gh: func(args []string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
			return "https://github.com/o/r/pull/6\n", nil
		}
		return "", nil
	}}
	s.git = s.git.WithCommander(ghf)

	w := do(t, s.Handler(), "POST", "/changes/"+id+"/close", ``)
	if w.Code != 502 {
		t.Fatalf("code = %d body = %s, want 502", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "review.md") {
		t.Errorf("body = %s, want the missing-review message", w.Body)
	}
	st, _ := s.git.GetState()
	if st[id].ReviewState != gitops.ReviewFailed {
		t.Errorf("reviewState = %q, want failed", st[id].ReviewState)
	}
	_ = wt
}

// doHTML issues a GET with Accept: text/html so handlers render the page
// instead of JSON.
func doHTML(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestWorktreeStripOnBoard(t *testing.T) {
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)

	// Active state: branch + remove button, no flags.
	w := doHTML(t, s.Handler(), "/changes/"+id)
	if w.Code != 200 {
		t.Fatalf("board: %d", w.Code)
	}
	for _, want := range []string{"worktree-strip", "change/" + id, "worktree-remove-btn"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("board HTML missing %q", want)
		}
	}
	if strings.Contains(w.Body.String(), "worktree missing") {
		t.Error("active worktree rendered as missing")
	}

	// A change without a worktree entry renders no strip.
	w = doHTML(t, s.Handler(), "/changes/2026-09-10-0")
	if strings.Contains(w.Body.String(), "worktree-strip") {
		t.Error("main-tree change rendered a worktree strip")
	}

	// Stale state (worktree deleted by hand) renders the missing flag and
	// the cleanup stays available.
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	w = doHTML(t, s.Handler(), "/changes/"+id)
	if !strings.Contains(w.Body.String(), "worktree missing") {
		t.Error("stale worktree not flagged as missing")
	}
}

func TestWorktreeRemove(t *testing.T) {
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)

	// Dirty worktree: refused with guidance.
	if err := os.WriteFile(filepath.Join(wt, "loose.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "POST", "/changes/"+id+"/worktree/remove", ``)
	if w.Code != 422 {
		t.Fatalf("dirty remove: %d %s, want 422", w.Code, w.Body)
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatal("dirty worktree was removed")
	}

	// Clean: removed along with its state entry.
	if err := os.Remove(filepath.Join(wt, "loose.txt")); err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "POST", "/changes/"+id+"/worktree/remove", ``)
	if w.Code != 200 {
		t.Fatalf("clean remove: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree still on disk after removal")
	}
	if st, _ := s.git.GetState(); len(st) != 0 {
		t.Errorf("state after removal = %v, want empty", st)
	}
	// Second call: no entry → 404.
	w = do(t, s.Handler(), "POST", "/changes/"+id+"/worktree/remove", ``)
	if w.Code != 404 {
		t.Fatalf("second remove: %d, want 404", w.Code)
	}
}

func TestWorktreeRemoveDisabled(t *testing.T) {
	s := gitFixtureServer(t, false, true)
	s.SetOpencode(nil)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/worktree/remove", ``)
	if w.Code != 409 {
		t.Fatalf("code = %d, want 409 when the pipeline is disabled", w.Code)
	}
}

func TestReviewerSessionPersistsAndBinds(t *testing.T) {
	cap := &ocCapture{}
	s := gitFixtureServer(t, true, true)
	fake := httptest.NewServer(cap.handler())
	t.Cleanup(fake.Close)
	s.SetOpencode(opencode.New(fake.URL, "pw"))
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)
	addBareRemote(t, s.st.Dir)
	commitWorktreeFile(t, wt, "changes/"+id+"/review.md", "## Verdict\nApprove")

	ghf := &ghFake{real: gitops.ExecCommander(), gh: func(args []string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
			return "https://github.com/o/r/pull/7\n", nil
		}
		return "", nil
	}}
	s.git = s.git.WithCommander(ghf)

	if w := do(t, s.Handler(), "POST", "/changes/"+id+"/close", ``); w.Code != 200 {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	// The reviewer is bound to the change and never deleted.
	entries := s.sessions.list(id)
	var bound bool
	for _, e := range entries {
		if e.Title == id+" — PR review" {
			bound = true
		}
	}
	if !bound {
		t.Errorf("reviewer session not bound; sessions = %+v", entries)
	}
	if cap.deletes != 0 {
		t.Errorf("deletes = %d, want 0 (the reviewer must stay)", cap.deletes)
	}

	// Re-close reuses the open PR (no second pr create) and re-runs the
	// reviewer without failing on "already exists".
	createsBefore := len(ghf.calls)
	if w := do(t, s.Handler(), "POST", "/changes/"+id+"/close", ``); w.Code != 200 {
		t.Fatalf("re-close: %d %s", w.Code, w.Body)
	}
	for _, call := range ghf.calls[createsBefore:] {
		if call[0] == "gh" && strings.Contains(call[1], "pr create") {
			t.Error("re-close created a second PR")
		}
	}
}

func TestReviewerPrimeFailureDeletesAndBlocks(t *testing.T) {
	cap := &ocCapture{failPrompts: true}
	s := gitFixtureServer(t, true, true)
	fake := httptest.NewServer(cap.handler())
	t.Cleanup(fake.Close)
	s.SetOpencode(opencode.New(fake.URL, "pw"))
	id, wt := scaffoldWorktreeChange(t, s)
	commitWorktreeAll(t, wt)
	addBareRemote(t, s.st.Dir)

	ghf := &ghFake{real: gitops.ExecCommander(), gh: func(args []string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
			return "https://github.com/o/r/pull/8\n", nil
		}
		return "", nil
	}}
	s.git = s.git.WithCommander(ghf)

	w := do(t, s.Handler(), "POST", "/changes/"+id+"/close", ``)
	if w.Code != 502 {
		t.Fatalf("code = %d body = %s, want 502", w.Code, w.Body)
	}
	if cap.deletes == 0 {
		t.Error("a reviewer that never primed must be deleted (nothing unbound leaks)")
	}
	st, _ := s.git.GetState()
	if st[id].ReviewState != gitops.ReviewFailed {
		t.Errorf("reviewState = %q, want failed", st[id].ReviewState)
	}
}

func TestReviewDetailModal(t *testing.T) {
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	id, wt := scaffoldWorktreeChange(t, s)

	// No review yet: HTML gets the friendly empty state, JSON a 404.
	w := doHTML(t, s.Handler(), "/changes/"+id+"/review")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "No review written yet") {
		t.Errorf("empty HTML = %d %s", w.Code, w.Body)
	}
	w = do(t, s.Handler(), "GET", "/changes/"+id+"/review", "")
	if w.Code != 404 {
		t.Errorf("missing JSON = %d, want 404", w.Code)
	}

	// Written: rendered as markdown with the TOC-bearing modal shape.
	commitWorktreeFile(t, wt, "changes/"+id+"/review.md", "# Verdict\n\nApprove — tests cover the diff.")
	w = doHTML(t, s.Handler(), "/changes/"+id+"/review")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Approve — tests cover the diff.") {
		t.Errorf("review HTML = %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "modal-toc") {
		t.Error("review modal is missing the TOC-bearing shape")
	}
	w = do(t, s.Handler(), "GET", "/changes/"+id+"/review", "")
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["body"], "Approve") {
		t.Errorf("JSON body = %v", resp)
	}
}
