package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"lessmess/internal/model"
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

func TestStatusEndpoint(t *testing.T) {
	s := mappingServer(t, nil)

	// Every allowed transition updates both the change ledger and the root row.
	for _, want := range []model.OverallStatus{model.OverallPlanned, model.OverallBlocked, model.OverallInProgress} {
		w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/status", `{"status":"`+string(want)+`"}`)
		if w.Code != 200 {
			t.Fatalf("status %s: code = %d body = %s", want, w.Code, w.Body)
		}
		c, _ := s.st.Change("2026-09-10-0")
		if c.Ledger.Overall != want {
			t.Fatalf("overall = %q, want %q", c.Ledger.Overall, want)
		}
		root, _ := s.st.Root()
		var rs model.OverallStatus
		for _, row := range root.Rows {
			if row.Change == "2026-09-10-0" {
				rs = row.Status
			}
		}
		if rs != want {
			t.Fatalf("root status = %q, want %q", rs, want)
		}
	}

	// Done is user-gated: the endpoint refuses it and points at close.
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/status", `{"status":"Done"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("Done: code = %d body = %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "/close") {
		t.Fatalf("Done: body should point at close, got %s", w.Body)
	}

	// Cancelled is out of scope; unknown and missing values are 422.
	for _, body := range []string{`{"status":"Cancelled"}`, `{"status":"Sideways"}`, `{}`} {
		if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/status", body); w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("body %s: code = %d, want 422", body, w.Code)
		}
	}
	// Malformed JSON is 400; unknown change is 404.
	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/status", `{`); w.Code != 400 {
		t.Fatalf("malformed: code = %d, want 400", w.Code)
	}
	if w := do(t, s.Handler(), "POST", "/changes/2099-01-01-9/status", `{"status":"Blocked"}`); w.Code != 404 {
		t.Fatalf("unknown change: code = %d, want 404", w.Code)
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
