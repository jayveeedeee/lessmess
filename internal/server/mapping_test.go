package server

import (
	"encoding/json"
	"fmt"
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
		"permanently bound to change 2026-09-10-0",
		"changes/2026-09-10-0/plan.md",
		"Current state (tool-injected; authoritative)",
		"NEVER edit .lessmess/workflow/ files by hand",
		"tasks/<task-id>/status",
		"NEVER create a new change directory",
		"/changes/scaffold",
		"offer a handoff",
		"only with the user's explicit approval",
		"changes/2026-09-10-0/spawn-change",
		`"session":"ses_new"`,
		"Delegation:",
		"prefix the task tool's description with the task's real ID",
		"becomes the subagent session's title verbatim",
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

func TestMappingTaskFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lessmess", "sessions.json")

	// A pre-change-shaped file (no task/parent keys) loads cleanly.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"2026-09-15-2":[{"session":"ses_old","title":"plain","created":"x"}]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := loadMapping(path)
	if err != nil {
		t.Fatalf("load old file: %v", err)
	}
	got := m.list("2026-09-15-2")
	if len(got) != 1 || got[0].Task != "" || got[0].Parent != "" {
		t.Fatalf("old entries = %+v", got)
	}

	// New-shaped entries round-trip with their annotations.
	if err := m.add("2026-09-15-2", SessionEntry{Session: "ses_sub", Title: "PSB-01: work", Created: "x", Task: "PSB-01", Parent: "ses_main"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string][]map[string]string
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var oldRaw, subRaw map[string]string
	for _, e := range raw["2026-09-15-2"] {
		if e["session"] == "ses_old" {
			oldRaw = e
		}
		if e["session"] == "ses_sub" {
			subRaw = e
		}
	}
	if oldRaw == nil {
		t.Fatal("old entry lost")
	}
	if _, ok := oldRaw["task"]; ok {
		t.Error("task key written for annotation-free entry")
	}
	if _, ok := oldRaw["parent"]; ok {
		t.Error("parent key written for annotation-free entry")
	}
	if subRaw == nil || subRaw["task"] != "PSB-01" || subRaw["parent"] != "ses_main" {
		t.Fatalf("sub entry = %+v", subRaw)
	}

	// Persistence across reload.
	m2, err := loadMapping(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got = m2.listByTask("2026-09-15-2", "PSB-01")
	if len(got) != 1 || got[0].Session != "ses_sub" || got[0].Parent != "ses_main" {
		t.Fatalf("listByTask = %+v", got)
	}
	if got := m2.listByTask("2026-09-15-2", "PSB-99"); len(got) != 0 {
		t.Fatalf("listByTask unknown = %+v", got)
	}
	// Empty task matches taskless entries.
	if got := m2.listByTask("2026-09-15-2", ""); len(got) != 1 || got[0].Session != "ses_old" {
		t.Fatalf("listByTask empty = %+v", got)
	}
}

func TestReconcileTaskSessions(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session":
			w.Write([]byte(`{"data":[
				{"id":"ses_main","title":"main","parentID":""},
				{"id":"ses_kid","title":"FIX-00: do the work","parentID":"ses_main","time":{"created":1789000000000}},
				{"id":"ses_weird","title":"no prefix here","parentID":"ses_main","time":{"created":1789000001000}},
				{"id":"ses_foreign","title":"FIX-00: elsewhere","parentID":"ses_other"},
				{"id":"ses_dup","title":"FIX-01: already mapped","parentID":"ses_main"},
				{"id":"ses_badtask","title":"NOPE-99: unknown task","parentID":"ses_main"}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	// ses_main is the change's bound session; ses_dup is already mapped.
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_main", Title: "main", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_dup", Title: "FIX-01: already mapped", Created: "x", Task: "FIX-01", Parent: "ses_main"}); err != nil {
		t.Fatal(err)
	}

	// Serving the session list reconciles the unmapped children.
	w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/sessions", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	entries := s.sessions.list("2026-09-10-0")
	byID := map[string]SessionEntry{}
	for _, e := range entries {
		byID[e.Session] = e
	}
	if len(entries) != 5 {
		t.Fatalf("entries = %+v", entries)
	}
	if e := byID["ses_kid"]; e.Task != "FIX-00" || e.Parent != "ses_main" {
		t.Errorf("ses_kid = %+v", e)
	}
	if e := byID["ses_weird"]; e.Task != "" || e.Parent != "ses_main" {
		t.Errorf("ses_weird = %+v", e)
	}
	if e := byID["ses_badtask"]; e.Task != "" {
		t.Errorf("ses_badtask = %+v", e)
	}
	if _, ok := byID["ses_foreign"]; ok {
		t.Error("child of another parent was mapped")
	}
	// Known tasks map exactly; created stamps come from the live session.
	if e := byID["ses_kid"]; !strings.HasPrefix(e.Created, "2026-") {
		t.Errorf("created = %q, want live stamp", e.Created)
	}

	// Serving again must not duplicate anything.
	w = do(t, s.Handler(), "GET", "/changes/2026-09-10-0/sessions", "")
	if w.Code != 200 {
		t.Fatalf("second code = %d body = %s", w.Code, w.Body)
	}
	if got := s.sessions.list("2026-09-10-0"); len(got) != 5 {
		t.Fatalf("after re-serve = %+v", got)
	}
}

func TestReconcileFailOpen(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_main", Title: "main", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/sessions", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	if got := s.sessions.list("2026-09-10-0"); len(got) != 1 {
		t.Fatalf("mapping changed on failed reconcile: %+v", got)
	}
}

func TestReconcileDeepPaginatedDescendantsDefensively(t *testing.T) {
	rootPages := 0
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		parent, cursor := r.URL.Query().Get("parentID"), r.URL.Query().Get("cursor")
		var sessions []map[string]any
		next := ""
		switch parent {
		case "ses_root":
			rootPages++
			start, end := 0, 50
			if cursor == "page-2" {
				start, end = 50, 51
			} else {
				next = "page-2"
			}
			for i := start; i < end; i++ {
				title := "hostile FIX-00: not a prefix"
				if i == 0 {
					title = "FIX-00: title must not replace binding"
				}
				sessions = append(sessions, map[string]any{"id": fmt.Sprintf("ses_child_%02d", i), "title": title, "parentID": parent, "time": map[string]int64{"created": int64(i + 1)}})
			}
			if cursor == "page-2" {
				sessions = append(sessions,
					map[string]any{"id": "ses_foreign", "title": "FIX-00: foreign", "parentID": parent},
					map[string]any{"id": "ses_hostile", "title": "FIX-00: wrong edge", "parentID": "ses_elsewhere"})
			}
		case "ses_child_01":
			sessions = append(sessions, map[string]any{"id": "ses_grand", "title": "FIX-01: deep", "parentID": parent, "time": map[string]int64{"created": 60}})
		case "ses_grand":
			// A hostile cycle back to an already visited root must terminate.
			sessions = append(sessions, map[string]any{"id": "ses_root", "title": "root", "parentID": parent})
		case "ses_foreign":
			sessions = append(sessions, map[string]any{"id": "ses_foreign_desc", "title": "FIX-00: do not claim", "parentID": parent})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": sessions, "cursor": map[string]string{"next": next}})
	})
	for change, entry := range map[string]SessionEntry{
		"2026-09-10-0": {Session: "ses_root", Title: "root", Created: "x"},
		"other-change": {Session: "ses_foreign", Title: "foreign", Created: "x"},
	} {
		if err := s.sessions.add(change, entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_child_00", Title: "old", Created: "x", Task: "FIX-01", Parent: "ses_root"}); err != nil {
		t.Fatal(err)
	}
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_dead", Title: "dead", Created: "x", Task: "FIX-00", Parent: "ses_root"}); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		w := do(t, s.Handler(), "GET", "/changes/2026-09-10-0/sessions", "")
		if w.Code != http.StatusOK {
			t.Fatalf("reconcile = %d %s", w.Code, w.Body.String())
		}
	}
	entries := s.sessions.list("2026-09-10-0")
	byID := map[string]SessionEntry{}
	for _, entry := range entries {
		byID[entry.Session] = entry
	}
	if len(entries) != 54 || rootPages < 4 {
		t.Fatalf("entries=%d root pages=%d", len(entries), rootPages)
	}
	if got := byID["ses_child_00"].Task; got != "FIX-01" {
		t.Fatalf("existing task binding replaced: %q", got)
	}
	if got := byID["ses_grand"]; got.Parent != "ses_child_01" || got.Task != "FIX-01" {
		t.Fatalf("deep child = %#v", got)
	}
	if _, ok := byID["ses_dead"]; !ok {
		t.Fatal("dead mapping was removed")
	}
	for _, id := range []string{"ses_foreign", "ses_foreign_desc", "ses_hostile"} {
		if _, ok := byID[id]; ok {
			t.Fatalf("unsafe session %s was claimed", id)
		}
	}
}
