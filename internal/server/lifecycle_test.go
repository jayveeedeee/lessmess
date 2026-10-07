package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/opencode"
)

func TestCloseReopenEndpoints(t *testing.T) {
	s := mappingServer(t, nil)

	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", `{}`)
	if w.Code != 200 {
		t.Fatalf("close code = %d body = %s", w.Code, w.Body)
	}
	c, _ := s.st.Change("2026-09-10-0")
	if c.Overall() != model.OverallDone {
		t.Fatalf("overall = %q", c.Overall())
	}

	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/reopen", `{}`)
	if w.Code != 200 {
		t.Fatalf("reopen code = %d body = %s", w.Code, w.Body)
	}
	c, _ = s.st.Change("2026-09-10-0")
	if c.Overall() != model.OverallInProgress {
		t.Fatalf("after reopen overall = %q", c.Overall())
	}

	if w := do(t, s.Handler(), "POST", "/changes/2099-01-01-9/close", `{}`); w.Code != 404 {
		t.Fatalf("unknown close: code = %d", w.Code)
	}
}

func TestCommitRequiresLiveWorktree(t *testing.T) {
	// Worktrees off: the gate refuses before any session is created, even
	// with a live service — the prompt's git add -A would sweep the whole
	// main tree.
	var created bool
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/session" {
			created = true
		}
		w.Write([]byte(`{"data":{"id":"ses_commit","title":"t","location":{"directory":"/x"}}}`))
	})
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/commit", `{}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("worktrees off: code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "the worktree pipeline is disabled" {
		t.Errorf("error = %q", resp["error"])
	}
	if created {
		t.Error("session created despite the worktree gate")
	}

	// Worktrees on, but the fixture change predates the feature: no
	// registered worktree → 404, still nothing spawned.
	ws := gitFixtureServer(t, true, true)
	cap := &ocCapture{}
	fake := httptest.NewServer(cap.handler())
	t.Cleanup(fake.Close)
	ws.SetOpencode(opencode.New(fake.URL, "pw"))
	w = do(t, ws.Handler(), "POST", "/changes/2026-09-10-0/commit", `{}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("no worktree entry: code = %d body = %s", w.Code, w.Body)
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "no live worktree registered for change 2026-09-10-0" {
		t.Errorf("error = %q", resp["error"])
	}
	if len(cap.creates) != 0 || len(cap.prompts) != 0 {
		t.Errorf("session spawned despite the gate: creates = %d, prompts = %d", len(cap.creates), len(cap.prompts))
	}
}

func TestCommitStatusEndpoint(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/wait") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/commit-status?session=ses_x", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]bool
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp["done"] {
		t.Fatalf("resp = %v, want done=true on 204", resp)
	}

	// Bad session id.
	if w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/commit-status?session=nope", ""); w.Code != 400 {
		t.Fatalf("bad session: code = %d", w.Code)
	}
}

func TestCommitStatusBusy(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/wait") {
			time.Sleep(5 * time.Second) // session still working; endpoint times out first
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	start := time.Now()
	w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/commit-status?session=ses_x", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("endpoint should time out ~2s, took %v", elapsed)
	}
	var resp map[string]bool
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["done"] {
		t.Fatalf("resp = %v, want done=false while busy", resp)
	}
}

func TestCommitStatusNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/commit-status?session=ses_x", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestCommitNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/commit", `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}
