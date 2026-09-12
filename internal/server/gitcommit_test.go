package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/opencode"
)

// requireGit skips the test when no git binary is available.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

// gitFixtureStore builds the standard fixture store inside an initialized
// git repository with one committed file. When dirty is true it then
// modifies the committed file and adds an untracked one.
func gitFixtureStore(t *testing.T, dirty bool) *Server {
	t.Helper()
	requireGit(t)
	st, dir := fixtureStore(t)

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "init")

	if dirty {
		if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := New(st)
	t.Cleanup(s.Close)
	return s
}

func TestGitStatusHelper(t *testing.T) {
	t.Run("not a repo", func(t *testing.T) {
		st := gitStatus(t.TempDir())
		if st.Repo {
			t.Fatal("Repo = true for non-git dir")
		}
	})

	t.Run("dirty repo", func(t *testing.T) {
		s := gitFixtureStore(t, true)
		st := gitStatus(s.st.Dir)
		if !st.Repo {
			t.Fatal("Repo = false for git dir")
		}
		var mod, untracked bool
		for _, c := range st.Changes {
			if c.Path == "tracked.txt" && strings.Contains(c.Code, "M") {
				mod = true
				// v1\n → v2\n: one line changed.
				if c.Added != "1" || c.Deleted != "1" {
					t.Fatalf("tracked.txt counts = +%s/-%s, want +1/-1", c.Added, c.Deleted)
				}
			}
			if c.Path == "untracked.txt" && c.Code == "??" {
				untracked = true
				if c.Added != "" || c.Deleted != "" {
					t.Fatalf("untracked.txt should have no counts, got +%s/-%s", c.Added, c.Deleted)
				}
			}
		}
		if !mod || !untracked {
			t.Fatalf("changes = %+v, want M tracked.txt and ?? untracked.txt", st.Changes)
		}
		if !strings.Contains(st.Summary, "1 file changed") {
			t.Fatalf("summary = %q, want shortstat line", st.Summary)
		}
	})

	t.Run("clean repo", func(t *testing.T) {
		s := gitFixtureStore(t, false)
		st := gitStatus(s.st.Dir)
		if !st.Repo || len(st.Changes) != 0 {
			t.Fatalf("clean repo: repo=%v changes=%+v", st.Repo, st.Changes)
		}
	})
}

func TestGitStatusEndpoint(t *testing.T) {
	s := mappingServer(t, nil) // fixture dir is not a git repo
	w := do(t, s.Handler(), "GET", "/api/git/status", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp gitRepoStatus
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Repo {
		t.Fatalf("resp = %+v, want repo:false", resp)
	}
}

func TestCommitAllEndpoint(t *testing.T) {
	var promptedText string
	s := gitFixtureStore(t, true)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_repocommit","title":"t","location":{"directory":"/x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			promptedText = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(fake.Close)
	s.SetOpencode(opencode.New(fake.URL, "pw"))

	w := do(t, s.Handler(), "POST", "/api/git/commit", `{}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["session"] != "ses_repocommit" {
		t.Fatalf("resp = %v", resp)
	}
	entries := s.sessions.listUnassigned()
	if len(entries) != 1 || entries[0].Session != "ses_repocommit" {
		t.Fatalf("unassigned mapping = %+v", entries)
	}
	for _, want := range []string{"git status", "git diff", "git add -A", "NEVER push"} {
		if !strings.Contains(promptedText, want) {
			t.Errorf("repo commit prompt missing %q", want)
		}
	}
	if strings.Contains(promptedText, "changes/2026-09-10-0") {
		t.Error("repo commit prompt references a specific change record")
	}
}

func TestCommitAllCleanTree(t *testing.T) {
	s := gitFixtureStore(t, false) // clean repo; oc nil but dirtiness check runs first
	w := do(t, s.Handler(), "POST", "/api/git/commit", `{}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d body = %s, want 422", w.Code, w.Body)
	}
	if got := len(s.sessions.listUnassigned()); got != 0 {
		t.Fatalf("unassigned sessions = %d, want 0 (no session on clean tree)", got)
	}
}

func TestCommitAllNotARepo(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/api/git/commit", `{}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d body = %s, want 422", w.Code, w.Body)
	}
}

func TestCommitAllNoService(t *testing.T) {
	s := gitFixtureStore(t, true) // dirty repo, oc nil → 503 after dirtiness check
	w := do(t, s.Handler(), "POST", "/api/git/commit", `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestIndexCommitAllButton(t *testing.T) {
	t.Run("dirty repo renders enabled button", func(t *testing.T) {
		s := gitFixtureStore(t, true)
		w := htmlGet(t, s.Handler(), "/", false)
		body := w.Body.String()
		if !strings.Contains(body, `id="commit-all-btn"`) {
			t.Fatal("commit-all-btn missing on dirty repo")
		}
		if strings.Contains(body, `id="commit-all-btn" disabled`) {
			t.Fatal("commit-all-btn disabled on dirty repo")
		}
	})

	t.Run("clean repo renders disabled button", func(t *testing.T) {
		s := gitFixtureStore(t, false)
		w := htmlGet(t, s.Handler(), "/", false)
		if !strings.Contains(w.Body.String(), `id="commit-all-btn" disabled`) {
			t.Fatal("commit-all-btn should be disabled on clean repo")
		}
	})

	t.Run("non-repo hides button", func(t *testing.T) {
		s := mappingServer(t, nil)
		w := htmlGet(t, s.Handler(), "/", false)
		if strings.Contains(w.Body.String(), "commit-all-btn") {
			t.Fatal("commit-all-btn should not render outside a git repo")
		}
	})
}

func TestRepoCommitStatusRoute(t *testing.T) {
	s := mappingServer(t, nil)
	if w := do(t, s.Handler(), "GET", "/api/git/commit-status?session=nope", ""); w.Code != 400 {
		t.Fatalf("bad session: code = %d, want 400", w.Code)
	}
	if w := do(t, s.Handler(), "GET", "/api/git/commit-status?session=ses_x", ""); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("no oc: code = %d, want 503", w.Code)
	}
}
