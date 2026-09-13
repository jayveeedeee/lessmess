package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/opencode"
)

func TestMappingUnassigned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lessmess", "sessions.json")
	m, _ := loadMapping(path)

	e := SessionEntry{Session: "ses_d", Title: "disc", Created: "2026-09-12T10:00:00Z"}
	if err := m.addUnassigned(e); err != nil {
		t.Fatal(err)
	}
	if got := m.listUnassigned(); len(got) != 1 || got[0].Session != "ses_d" {
		t.Fatalf("unassigned = %+v", got)
	}

	// Move to a change; persists across reload.
	moved, err := m.moveToChange("ses_d", "2026-09-12-6")
	if err != nil || !moved {
		t.Fatalf("move = %v, %v", moved, err)
	}
	m2, _ := loadMapping(path)
	if len(m2.listUnassigned()) != 0 {
		t.Fatal("bucket not emptied")
	}
	if got := m2.list("2026-09-12-6"); len(got) != 1 || got[0].Session != "ses_d" {
		t.Fatalf("change sessions = %+v", got)
	}

	// Idempotent: second move reports false without error.
	if moved, err := m2.moveToChange("ses_d", "2026-09-12-6"); err != nil || moved {
		t.Fatalf("re-move = %v, %v", moved, err)
	}
}

func TestMappingCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lessmess", "sessions.json")

	m, err := loadMapping(path)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if len(m.list("2026-09-12-0")) != 0 {
		t.Fatal("expected empty mapping")
	}
	e1 := SessionEntry{Session: "ses_1", Title: "one", Created: "2026-09-12T10:00:00Z"}
	e2 := SessionEntry{Session: "ses_2", Title: "two", Created: "2026-09-12T10:01:00Z"}
	if err := m.add("2026-09-12-0", e1); err != nil {
		t.Fatal(err)
	}
	if err := m.add("2026-09-12-0", e2); err != nil {
		t.Fatal(err)
	}

	// Reload from disk: persistence.
	m2, err := loadMapping(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := m2.list("2026-09-12-0"); len(got) != 2 || got[1].Session != "ses_2" {
		t.Fatalf("list = %+v", got)
	}

	found, err := m2.remove("2026-09-12-0", "ses_1")
	if err != nil || !found {
		t.Fatalf("remove = %v, %v", found, err)
	}
	if got := m2.list("2026-09-12-0"); len(got) != 1 || got[0].Session != "ses_2" {
		t.Fatalf("after remove = %+v", got)
	}
	if found, _ := m2.remove("2026-09-12-0", "ses_nope"); found {
		t.Fatal("remove of unknown should report false")
	}
}

func TestMappingCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if _, err := loadMapping(path); err == nil {
		t.Fatal("expected error for corrupt file")
	}
}

func TestMappingChangeOf(t *testing.T) {
	dir := t.TempDir()
	m, _ := loadMapping(filepath.Join(dir, ".lessmess", "sessions.json"))
	m.addUnassigned(SessionEntry{Session: "ses_d", Title: "d", Created: "x"})
	m.add("2026-09-10-0", SessionEntry{Session: "ses_a", Title: "a", Created: "x"})

	// Unassigned bucket is not a change.
	if _, ok := m.changeOf("ses_d"); ok {
		t.Fatal("unassigned session reported as bound")
	}
	if got, ok := m.changeOf("ses_a"); !ok || got != "2026-09-10-0" {
		t.Fatalf("changeOf = %q, %v", got, ok)
	}
	if _, ok := m.changeOf("ses_nope"); ok {
		t.Fatal("unknown session reported as bound")
	}
}

func mappingServer(t *testing.T, ocHandler http.HandlerFunc) *Server {
	t.Helper()
	st, _ := fixtureStore(t)
	s := New(st)
	t.Cleanup(s.Close)
	if ocHandler != nil {
		fake := httptest.NewServer(ocHandler)
		t.Cleanup(fake.Close)
		s.SetOpencode(opencode.New(fake.URL, "pw"))
	}
	return s
}

func TestSessionChangeEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_bound", Title: "t", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	// Bound session → its change.
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_bound/change", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["change"] != "2026-09-10-0" {
		t.Errorf("resp = %v, want change 2026-09-10-0", resp)
	}
	// Unknown session → empty (unassigned discussions answer the same).
	w = do(t, s.Handler(), "GET", "/api/sessions/ses_nope/change", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["change"] != "" {
		t.Errorf("resp = %v, want empty change", resp)
	}
}

func TestListSessionsEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	// Seed the mapping file directly.
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_stored", Title: "stored title", Created: "2026-09-12T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	// No opencode client: falls back to stored title.
	w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/sessions", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp struct {
		Sessions []sessionResponse `json:"sessions"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Sessions) != 1 || resp.Sessions[0].Title != "stored title" || resp.Sessions[0].Live {
		t.Fatalf("resp = %+v", resp.Sessions)
	}
}

func TestCreateSessionEndpoint(t *testing.T) {
	var promptedText string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session" && r.Method == http.MethodPost {
			w.Write([]byte(`{"data":{"id":"ses_new","title":"t","location":{"directory":"/x"}}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/prompt") {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			promptedText = body["text"]
			w.Write([]byte(`{"data":{}}`))
			return
		}
		if r.URL.Path == "/api/session/ses_stored" {
			w.Write([]byte(`{"data":{"id":"ses_stored","title":"live title","location":{"directory":"/x"}}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	// Create.
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/sessions", `{"title":"my session"}`)
	if w.Code != 201 {
		t.Fatalf("create code = %d body = %s", w.Code, w.Body)
	}
	// Mapping file persisted.
	entries := s.sessions.list("2026-09-10-0")
	if len(entries) != 1 || entries[0].Session != "ses_new" || entries[0].Title != "my session" {
		t.Fatalf("mapping = %+v", entries)
	}
	// Primed with the change-scoped prompt.
	for _, want := range []string{
		"bound to change 2026-09-10-0",
		"changes/2026-09-10-0/plan.md",
		"changes/2026-09-10-0/ledger.md",
		"existing task-ID prefix",
		"NEVER create a new change directory",
		"/changes/scaffold",
		"new discussion from the index page",
	} {
		if !strings.Contains(promptedText, want) {
			t.Errorf("prompt missing %q:\n%s", want, promptedText)
		}
	}
	// List enriches from the live service.
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_stored", Title: "stored", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "GET", "/changes/2026-09-10-0/sessions", "")
	var resp struct {
		Sessions []sessionResponse `json:"sessions"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	var enriched *sessionResponse
	for i := range resp.Sessions {
		if resp.Sessions[i].Session == "ses_stored" {
			enriched = &resp.Sessions[i]
		}
	}
	if enriched == nil || enriched.Title != "live title" || !enriched.Live {
		t.Fatalf("enriched = %+v", enriched)
	}
}

func TestCreateSessionNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/sessions", `{"title":"x"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestCreateSessionPrimeFailure(t *testing.T) {
	var deleted bool
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session" && r.Method == http.MethodPost {
			w.Write([]byte(`{"data":{"id":"ses_p","title":"t","location":{"directory":"/x"}}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/prompt") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/session/ses_p" {
			deleted = true
			w.Write([]byte(`{"data":{}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/sessions", `{"title":"x"}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502; body = %s", w.Code, w.Body)
	}
	if !deleted {
		t.Fatal("unprimed session was not deleted")
	}
	if got := s.sessions.list("2026-09-10-0"); len(got) != 0 {
		t.Fatalf("mapping = %+v, want empty", got)
	}
}

func TestUnlinkSessionEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_1", Title: "t", Created: "x"})
	req, _ := http.NewRequest("DELETE", "/changes/2026-09-10-0/sessions/ses_1", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	if len(s.sessions.list("2026-09-10-0")) != 0 {
		t.Fatal("mapping not updated")
	}
	// Unknown session → 404.
	req2, _ := http.NewRequest("DELETE", "/changes/2026-09-10-0/sessions/ses_nope", nil)
	w2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(w2, req2)
	if w2.Code != 404 {
		t.Fatalf("code = %d, want 404", w2.Code)
	}
}

func TestListDiscussionsEndpoint(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session/ses_d" {
			w.Write([]byte(`{"data":{"id":"ses_d","title":"live discussion","location":{"directory":"/x"}}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	if err := s.sessions.addUnassigned(SessionEntry{Session: "ses_d", Title: "stored", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "GET", "/api/discussions", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp struct {
		Sessions []sessionResponse `json:"sessions"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Sessions) != 1 || resp.Sessions[0].Title != "live discussion" || !resp.Sessions[0].Live {
		t.Fatalf("resp = %+v", resp.Sessions)
	}
}

func TestMappingPathUsesToolingDir(t *testing.T) {
	st, dir := fixtureStore(t)
	s := New(st)
	t.Cleanup(s.Close)
	if !strings.HasSuffix(s.sessions.path, filepath.Join(".lessmess", "sessions.json")) {
		t.Fatalf("path = %s", s.sessions.path)
	}
	if !strings.HasPrefix(s.sessions.path, dir) {
		t.Fatalf("path = %s not under repo %s", s.sessions.path, dir)
	}
}
