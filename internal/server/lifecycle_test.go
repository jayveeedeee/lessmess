package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"tasktracker/internal/model"
)

func TestCloseReopenEndpoints(t *testing.T) {
	s := mappingServer(t, nil)

	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", `{}`)
	if w.Code != 200 {
		t.Fatalf("close code = %d body = %s", w.Code, w.Body)
	}
	c, _ := s.st.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallDone {
		t.Fatalf("overall = %q", c.Ledger.Overall)
	}
	root, _ := s.st.Root()
	if root.Rows[0].Status != model.OverallDone {
		t.Fatalf("root status = %q", root.Rows[0].Status)
	}

	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/reopen", `{}`)
	if w.Code != 200 {
		t.Fatalf("reopen code = %d body = %s", w.Code, w.Body)
	}
	c, _ = s.st.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallInProgress {
		t.Fatalf("after reopen overall = %q", c.Ledger.Overall)
	}

	if w := do(t, s.Handler(), "POST", "/changes/2099-01-01-9/close", `{}`); w.Code != 404 {
		t.Fatalf("unknown close: code = %d", w.Code)
	}
}

func TestCommitEndpoint(t *testing.T) {
	var promptedText string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_commit","title":"t","location":{"directory":"/x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			promptedText = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/commit", `{}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["session"] != "ses_commit" {
		t.Fatalf("resp = %v", resp)
	}
	entries := s.sessions.list("2026-09-10-0")
	if len(entries) != 1 || entries[0].Session != "ses_commit" {
		t.Fatalf("mapping = %+v", entries)
	}
	for _, want := range []string{"git status", "git diff", "commit message", "git add -A", "NEVER push"} {
		if !strings.Contains(promptedText, want) {
			t.Errorf("commit prompt missing %q", want)
		}
	}
}

func TestCommitNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/commit", `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}
