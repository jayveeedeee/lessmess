package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDiscussionSession(t *testing.T) {
	var promptedText string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_disc","title":"t","location":{"directory":"/x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			promptedText = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	s.PublicBase = "127.0.0.1:9090"

	w := do(t, s.Handler(), "POST", "/changes/session", `{}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp sessionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Session != "ses_disc" {
		t.Fatalf("resp = %+v", resp)
	}

	// Mapped to the unassigned bucket.
	if got := s.sessions.listUnassigned(); len(got) != 1 || got[0].Session != "ses_disc" {
		t.Fatalf("unassigned = %+v", got)
	}

	// Prompt: discussion-only + exact scaffold call with injected base and session ID.
	for _, want := range []string{
		"AGENTS.md",
		"plan a NEW change",
		"Empty state",
		"do NOT investigate the repository",
		`"What would you like to build?"`,
		"DO NOT modify the repository",
		"EXPLICITLY agrees",
		"curl -s -X POST http://127.0.0.1:9090/changes/scaffold",
		`"session":"ses_disc"`,
		"task-ID prefix",
	} {
		if !strings.Contains(promptedText, want) {
			t.Errorf("prompt missing %q:\n%s", want, promptedText)
		}
	}
}

func TestDiscussionSessionNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/changes/session", `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestScaffoldFlow(t *testing.T) {
	var renamedTo string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/rename") {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			renamedTo = body["title"]
			w.Write([]byte(`{"data":{}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_sc", Title: "disc", Created: "x"}); err != nil {
		t.Fatal(err)
	}

	w := do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"Build the thing","prefix":"BT","session":"ses_sc"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	changeID := resp["change"]
	if changeID == "" {
		t.Fatalf("resp = %v", resp)
	}

	// Change exists and root row carries title + prefix.
	if w := do(t, s.Handler(), "GET", "/changes/"+changeID, ""); w.Code != 200 {
		t.Fatalf("board: code = %d", w.Code)
	}
	root, err := s.st.Root()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range root.Rows {
		if r.Change == changeID {
			found = true
			if r.Title != "Build the thing" || r.Prefix != "BT" {
				t.Fatalf("root row = %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("root row missing for scaffolded change")
	}

	// Session renamed and mapping moved out of the bucket.
	if renamedTo != changeID+" — Build the thing" {
		t.Fatalf("renamedTo = %q", renamedTo)
	}
	if len(s.sessions.listUnassigned()) != 0 {
		t.Fatal("bucket not emptied")
	}
	if got := s.sessions.list(changeID); len(got) != 1 || got[0].Session != "ses_sc" {
		t.Fatalf("change sessions = %+v", got)
	}

	// The produced change validates clean.
	if v := s.st.Validate(); len(v) != 0 {
		t.Fatalf("violations after scaffold: %v", v)
	}
}

func TestScaffoldValidation(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {})
	for body, want := range map[string]int{
		`{"title":"","session":"ses_x"}`:                       422,
		`{"title":"a|b","session":"ses_x"}`:                    422,
		`{"title":"x","prefix":"abc","session":"ses_x"}`:       422,
		`{"title":"x","prefix":"TOOLONG","session":"ses_x"}`:   422,
		`{"title":"x","session":"nope"}`:                       422,
		`{"title":"x","prefix":"OK","session":"ses_ok"}`:       201,
		`{"title":"y","prefix":"","session":"ses_ok2"}`:        201,
	} {
		if w := do(t, s.Handler(), "POST", "/changes/scaffold", body); w.Code != want {
			t.Errorf("%s → %d, want %d", body, w.Code, want)
		}
	}
}

func TestScaffoldIdempotentMove(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{}}`))
	})
	// Session is NOT in the unassigned bucket (retry or external session).
	w := do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"Direct link","prefix":"DL","session":"ses_ext"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if got := s.sessions.list(resp["change"]); len(got) != 1 || got[0].Session != "ses_ext" {
		t.Fatalf("change sessions = %+v", got)
	}
}

func TestScaffoldBoundSessionRefused(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{}}`))
	})
	// Session already belongs to an existing change.
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_bound", Title: "t", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	rootBefore, err := s.st.Root()
	if err != nil {
		t.Fatal(err)
	}

	w := do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"Sneaky new change","prefix":"SN","session":"ses_bound"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409; body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["change"] != "2026-09-10-0" {
		t.Fatalf("resp = %v", resp)
	}
	if !strings.Contains(resp["error"], "continue the work within that change") {
		t.Fatalf("error not agent-redirecting: %v", resp["error"])
	}

	// Nothing created; mapping untouched.
	rootAfter, err := s.st.Root()
	if err != nil {
		t.Fatal(err)
	}
	if len(rootAfter.Rows) != len(rootBefore.Rows) {
		t.Fatalf("root rows grew: %d → %d", len(rootBefore.Rows), len(rootAfter.Rows))
	}
	if got := s.sessions.list("2026-09-10-0"); len(got) != 1 || got[0].Session != "ses_bound" {
		t.Fatalf("change sessions = %+v", got)
	}
	if v := s.st.Validate(); len(v) != 0 {
		t.Fatalf("violations after refused scaffold: %v", v)
	}
}
