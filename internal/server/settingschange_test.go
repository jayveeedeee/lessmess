package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// scFake is a fake opencode service for the settings-change flow.
type scFake struct {
	creates   int
	prompts   []string
	deadGet   bool // GET /api/session/{id} answers 404
	lastTitle string
}

func (f *scFake) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			f.creates++
			id := fmt.Sprintf("ses_sc%d", f.creates)
			f.lastTitle = ""
			w.Write([]byte(`{"data":{"id":"` + id + `","title":"t","location":{"directory":"/x"}}}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/session/ses_") && !strings.Contains(r.URL.Path[len("/api/session/"):], "/"):
			if f.deadGet {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"no such session"}`))
				return
			}
			w.Write([]byte(`{"data":{"id":"ses_x","title":"existing settings discussion"}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			f.prompts = append(f.prompts, body["text"])
			w.Write([]byte(`{"data":{}}`))
		default:
			w.Write([]byte(`{"data":{}}`))
		}
	}
}

func postSettingsChange(t *testing.T, s *Server, body string) (int, map[string]any) {
	t.Helper()
	w := do(t, s.Handler(), "POST", "/api/settings/change", body)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

func TestSettingsChangeFreshDiscussion(t *testing.T) {
	fake := &scFake{}
	s := mappingServer(t, fake.handler())
	writeSettingsFile(t, s.st.Dir, Settings{Session: SessionSettings{Agent: "build"}})

	code, resp := postSettingsChange(t, s, `{"field":"session.agent","scope":"project"}`)
	if code != 201 {
		t.Fatalf("code = %d resp = %v", code, resp)
	}
	if resp["reused"] != false || resp["session"] != "ses_sc1" {
		t.Errorf("resp = %v", resp)
	}
	if fake.creates != 1 || len(fake.prompts) != 1 {
		t.Fatalf("creates = %d prompts = %d", fake.creates, len(fake.prompts))
	}
	p := fake.prompts[0]
	for _, want := range []string{"planning assistant", "Settings page", "session.agent", "build", "source: project", "scope when clicked: project"} {
		if !strings.Contains(p, want) {
			t.Errorf("prime missing %q", want)
		}
	}
	// State + mapping recorded.
	if st := loadSettingsChangeState(s.st.Dir); st.Session != "ses_sc1" {
		t.Errorf("state = %+v", st)
	}
	if _, ok := s.sessions.changeOf("ses_sc1"); ok {
		t.Error("fresh discussion must stay unassigned until scaffolded")
	}
}

func TestSettingsChangeReusesOpenDiscussion(t *testing.T) {
	fake := &scFake{}
	s := mappingServer(t, fake.handler())

	postSettingsChange(t, s, `{"field":"session.agent","scope":"project"}`)
	code, resp := postSettingsChange(t, s, `{"field":"ui.showArchived","scope":"personal"}`)
	if code != 200 {
		t.Fatalf("code = %d resp = %v", code, resp)
	}
	if resp["reused"] != true || resp["session"] != "ses_sc1" {
		t.Errorf("resp = %v, want reuse of ses_sc1", resp)
	}
	if fake.creates != 1 {
		t.Errorf("creates = %d, want 1 (no new session)", fake.creates)
	}
	if len(fake.prompts) != 2 {
		t.Fatalf("prompts = %d", len(fake.prompts))
	}
	fold := fake.prompts[1]
	for _, want := range []string{"another setting", "ui.showArchived", "true", "source: default", "scope when clicked: personal"} {
		if !strings.Contains(fold, want) {
			t.Errorf("fold-in missing %q: %q", want, fold[:120])
		}
	}
	// No base prompt in the fold-in message.
	if strings.Contains(fold, "planning assistant") {
		t.Error("reuse message must not re-prime the discussion prompt")
	}
}

func TestSettingsChangeClosedChangeStartsFresh(t *testing.T) {
	fake := &scFake{}
	s := mappingServer(t, fake.handler())

	// State session bound to the fixture change, which is then closed.
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_old", Title: "t", Created: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := saveSettingsChangeState(s.st.Dir, settingsChangeState{Session: "ses_old"}); err != nil {
		t.Fatal(err)
	}
	if err := s.st.SetChangeStatus("2026-09-10-0", model.OverallDone); err != nil {
		t.Fatal(err)
	}

	code, resp := postSettingsChange(t, s, `{"field":"git.defaultBranch","scope":"project"}`)
	if code != 201 || resp["reused"] != false {
		t.Fatalf("code = %d resp = %v, want a fresh discussion for a closed change", code, resp)
	}
	if fake.creates != 1 {
		t.Errorf("creates = %d, want 1", fake.creates)
	}
	if st := loadSettingsChangeState(s.st.Dir); st.Session != "ses_sc1" {
		t.Errorf("state = %+v, want the new session", st)
	}
}

func TestSettingsChangeDeadSessionStartsFresh(t *testing.T) {
	fake := &scFake{deadGet: true}
	s := mappingServer(t, fake.handler())
	if err := saveSettingsChangeState(s.st.Dir, settingsChangeState{Session: "ses_gone"}); err != nil {
		t.Fatal(err)
	}

	code, resp := postSettingsChange(t, s, `{"field":"docs.autoGardenerOnClose","scope":"project"}`)
	if code != 201 || resp["reused"] != false {
		t.Fatalf("code = %d resp = %v, want fresh after dead session", code, resp)
	}
}

func TestSettingsChangeRejects(t *testing.T) {
	fake := &scFake{}
	s := mappingServer(t, fake.handler())

	if code, resp := postSettingsChange(t, s, `{"field":"nope.field","scope":"project"}`); code != 422 {
		t.Errorf("unknown field: code = %d resp = %v, want 422", code, resp)
	}
	if code, _ := postSettingsChange(t, s, `{"field":"session.agent","scope":"nowhere"}`); code != 400 {
		t.Errorf("bad scope: code = %d, want 400", code)
	}
	if fake.creates != 0 {
		t.Errorf("rejected requests must not create sessions (creates = %d)", fake.creates)
	}

	// No opencode service → 503.
	s2 := mappingServer(t, nil)
	if code, _ := postSettingsChange(t, s2, `{"field":"session.agent","scope":"project"}`); code != 503 {
		t.Errorf("no service: code = %d, want 503", code)
	}
}
