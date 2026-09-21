package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

	// Prompt: read-only before scaffold, then execution-capable after approval,
	// with the exact scaffold call and injected base/session ID.
	for _, want := range []string{
		"AGENTS.md",
		"planning a NEW change",
		"Empty state",
		"do NOT investigate the repository",
		`"What would you like to build?"`,
		"Before scaffolding, do not modify the repository",
		"EXPLICITLY agrees",
		"continue in this same session",
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
		if r.Method == http.MethodPatch && r.URL.Path == "/api/session/ses_sc" {
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

	// Change exists and index entry carries title + prefix.
	if w := do(t, s.Handler(), "GET", "/changes/"+changeID, ""); w.Code != 200 {
		t.Fatalf("board: code = %d", w.Code)
	}
	e := s.st.Entry(changeID)
	if e == nil {
		t.Fatal("index entry missing for scaffolded change")
	}
	if e.Title != "Build the thing" || e.Prefix != "BT" {
		t.Fatalf("index entry = %+v", e)
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
		`{"title":"","session":"ses_x"}`:                     422,
		`{"title":"a|b","session":"ses_x"}`:                  422,
		`{"title":"x","prefix":"abc","session":"ses_x"}`:     422,
		`{"title":"x","prefix":"TOOLONG","session":"ses_x"}`: 422,
		`{"title":"x","session":"nope"}`:                     422,
		`{"title":"x","prefix":"OK","session":"ses_ok"}`:     201,
		`{"title":"y","prefix":"","session":"ses_ok2"}`:      201,
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
	idxBefore, err := s.st.Index()
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
	// The refusal teaches old-primed bound sessions the sanctioned handoff.
	for _, want := range []string{
		"changes/2026-09-10-0/handoff-<topic>.md",
		"POST /changes/2026-09-10-0/spawn-change",
		`{"title","prefix","artifact","session"}`,
		"explicit user approval",
	} {
		if !strings.Contains(resp["error"], want) {
			t.Errorf("409 missing %q: %v", want, resp["error"])
		}
	}

	// Nothing created; mapping untouched.
	idxAfter, err := s.st.Index()
	if err != nil {
		t.Fatal(err)
	}
	if len(idxAfter.Changes) != len(idxBefore.Changes) {
		t.Fatalf("index grew: %d → %d", len(idxBefore.Changes), len(idxAfter.Changes))
	}
	if got := s.sessions.list("2026-09-10-0"); len(got) != 1 || got[0].Session != "ses_bound" {
		t.Fatalf("change sessions = %+v", got)
	}
	if v := s.st.Validate(); len(v) != 0 {
		t.Fatalf("violations after refused scaffold: %v", v)
	}
}

func TestBindTaskSession(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/session/ses_kid":
			w.Write([]byte(`{"data":{"id":"ses_kid","title":"FIX-00: do the work","parentID":"ses_main"}}`))
		case r.URL.Path == "/api/session/ses_taken":
			w.Write([]byte(`{"data":{"id":"ses_taken","title":"elsewhere","parentID":"ses_other"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// Happy path: no caller session, sub exists, task exists.
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-00","sub":"ses_kid"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	entries := s.sessions.list("2026-09-10-0")
	if len(entries) != 1 || entries[0].Session != "ses_kid" || entries[0].Task != "FIX-00" || entries[0].Parent != "ses_main" {
		t.Fatalf("entries = %+v", entries)
	}

	// Idempotent exact retry → 200 reused.
	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-00","sub":"ses_kid"}`)
	if w.Code != 200 {
		t.Fatalf("retry code = %d body = %s", w.Code, w.Body)
	}

	// Rebinding to a different task → 409.
	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-01","sub":"ses_kid"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("rebind code = %d body = %s", w.Code, w.Body)
	}

	// Unknown task → 422.
	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"NOPE-99","sub":"ses_kid"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown task code = %d body = %s", w.Code, w.Body)
	}

	// Dead sub → 422.
	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-01","sub":"ses_ghost"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("dead sub code = %d body = %s", w.Code, w.Body)
	}

	// A supplied caller bound elsewhere → 409 naming that change.
	if err := s.sessions.add("2026-09-09-9", SessionEntry{Session: "ses_out", Title: "x", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-01","sub":"ses_kid","session":"ses_out"}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "2026-09-09-9") {
		t.Fatalf("caller code = %d body = %s", w.Code, w.Body)
	}

	// A sub already mapped to another change → 409.
	if err := s.sessions.add("2026-09-09-9", SessionEntry{Session: "ses_taken", Title: "x", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-01","sub":"ses_taken"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("taken sub code = %d body = %s", w.Code, w.Body)
	}

	// Unknown change → 404.
	w = do(t, s.Handler(), "POST", "/changes/1999-01-01-0/task-sessions", `{"task":"FIX-00","sub":"ses_kid"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown change code = %d body = %s", w.Code, w.Body)
	}
}

func TestBindTaskSessionNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-00","sub":"ses_kid"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestSpawnChangeHandoff(t *testing.T) {
	var prompted string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_new","title":"t","location":{"directory":"/x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			prompted = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	s.PublicBase = "127.0.0.1:9090"
	// Handoff artifact inside the source change; caller bound to it.
	artifact := filepath.Join(s.st.Dir, "changes", "2026-09-10-0", "handoff-resilience.md")
	if err := os.WriteFile(artifact, []byte("# context\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_main", Title: "t", Created: "x"}); err != nil {
		t.Fatal(err)
	}

	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/spawn-change",
		`{"title":"Client resilience","prefix":"CR","artifact":"handoff-resilience.md","session":"ses_main"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	changeID := resp["change"]
	if changeID == "" || resp["session"] != "ses_new" {
		t.Fatalf("resp = %v", resp)
	}

	// The new change exists, carries the title/prefix, and validates clean.
	e := s.st.Entry(changeID)
	if e == nil {
		t.Fatal("index entry missing for spawned change")
	}
	if e.Title != "Client resilience" || e.Prefix != "CR" {
		t.Fatalf("index entry = %+v", e)
	}
	if v := s.st.Validate(); len(v) != 0 {
		t.Fatalf("violations after spawn: %v", v)
	}

	// The fresh session is bound to the new change with provenance; the
	// source change's mapping is untouched.
	entries := s.sessions.list(changeID)
	if len(entries) != 1 || entries[0].Session != "ses_new" || entries[0].SpawnedFrom != "2026-09-10-0" {
		t.Fatalf("entries = %+v", entries)
	}
	if got := s.sessions.list("2026-09-10-0"); len(got) != 1 || got[0].Session != "ses_main" {
		t.Fatalf("source sessions = %+v", got)
	}

	// The prime is the change prompt plus the handoff addendum naming the
	// artifact, the source change, and the exact endpoint call.
	for _, want := range []string{
		"change execution assistant",
		"permanently bound to change " + changeID,
		"This session was spawned by a handoff from change 2026-09-10-0",
		"changes/2026-09-10-0/handoff-resilience.md",
		"authoritative starting context",
		"curl -s -X POST http://127.0.0.1:9090/changes/" + changeID + "/spawn-change",
		`"session":"ses_new"`,
	} {
		if !strings.Contains(prompted, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompted)
		}
	}

	// The sessions listing serializes provenance for the board badge.
	w = do(t, s.Handler(), "GET", "/changes/"+changeID+"/sessions", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"spawnedFrom":"2026-09-10-0"`) {
		t.Fatalf("sessions listing = %d %s", w.Code, w.Body)
	}
}

func TestListHandoffs(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {})
	cdir := filepath.Join(s.st.Dir, "changes", "2026-09-10-0")
	for _, f := range []string{"handoff-b.md", "handoff-a.md", "plan.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(cdir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/handoffs", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	var resp struct {
		Handoffs []string `json:"handoffs"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Handoffs) != 2 || resp.Handoffs[0] != "handoff-a.md" || resp.Handoffs[1] != "handoff-b.md" {
		t.Fatalf("handoffs = %+v", resp.Handoffs)
	}

	// No artifacts → empty list, not an error.
	if err := os.Remove(filepath.Join(cdir, "handoff-a.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(cdir, "handoff-b.md")); err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "GET", "/changes/2026-09-10-0/handoffs", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"handoffs":[]`) {
		t.Fatalf("empty handoffs = %d %s", w.Code, w.Body)
	}

	// Unknown change → 404.
	if w := do(t, s.Handler(), "GET", "/changes/1999-01-01-0/handoffs", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown change code = %d, want 404", w.Code)
	}
}

func TestSpawnChangeValidation(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{}}`))
	})
	if err := os.WriteFile(filepath.Join(s.st.Dir, "changes", "2026-09-10-0", "handoff-a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.sessions.add("2026-09-09-9", SessionEntry{Session: "ses_out", Title: "x", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	for body, want := range map[string]int{
		`{"title":"","artifact":"handoff-a.md"}`:                            422,
		`{"title":"a|b","artifact":"handoff-a.md"}`:                         422,
		`{"title":"x","prefix":"abc","artifact":"handoff-a.md"}`:            422,
		`{"title":"x","artifact":""}`:                                       422,
		`{"title":"x","artifact":"../plan.md"}`:                             422,
		`{"title":"x","artifact":"sub/handoff-a.md"}`:                       422,
		`{"title":"x","artifact":"plan.md"}`:                                422,
		`{"title":"x","artifact":"handoff-ghost.md"}`:                       422,
		`{"title":"x","artifact":"handoff-a.md","session":"ses_out"}`:       409,
		`{"title":"ok","prefix":"OK","artifact":"handoff-a.md"}`:            201,
		`{"title":"ok2","artifact":"handoff-a.md","session":"ses_unbound"}`: 201,
	} {
		if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/spawn-change", body); w.Code != want {
			t.Errorf("%s → %d, want %d", body, w.Code, want)
		}
	}

	// Unknown change → 404.
	if w := do(t, s.Handler(), "POST", "/changes/1999-01-01-0/spawn-change", `{"title":"x","artifact":"handoff-a.md"}`); w.Code != http.StatusNotFound {
		t.Errorf("unknown change code = %d, want 404", w.Code)
	}

	// Bad JSON → 400.
	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/spawn-change", `not json`); w.Code != http.StatusBadRequest {
		t.Errorf("bad json code = %d, want 400", w.Code)
	}
}

func TestSpawnChangeNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/spawn-change", `{"title":"x","artifact":"handoff-a.md"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestSpawnChangePrimeFailure(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_new","title":"t","location":{"directory":"/x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if err := os.WriteFile(filepath.Join(s.st.Dir, "changes", "2026-09-10-0", "handoff-a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/spawn-change", `{"title":"Client resilience","artifact":"handoff-a.md"}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	changeID := resp["change"]
	if changeID == "" {
		t.Fatalf("502 must name the created change: %v", resp)
	}

	// The change directory is canonical and survives; nothing stays bound.
	if w := do(t, s.Handler(), "GET", "/changes/"+changeID, ""); w.Code != 200 {
		t.Fatalf("board: code = %d", w.Code)
	}
	if got := s.sessions.list(changeID); len(got) != 0 {
		t.Fatalf("change sessions after failure = %+v", got)
	}
	if v := s.st.Validate(); len(v) != 0 {
		t.Fatalf("violations after failed spawn: %v", v)
	}
}
