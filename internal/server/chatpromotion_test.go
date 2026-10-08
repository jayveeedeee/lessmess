package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPromoteChatAdmissionAndScaffold(t *testing.T) {
	var sent atomic.Int32
	var prompt, synthetic string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}":{"patch":{}},"/api/session/{sessionID}/synthetic":{"post":{}}}}`))
		case r.URL.Path == "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body struct {
				Text   string `json:"text"`
				Skills []struct {
					ID string `json:"id"`
				} `json:"skills"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			prompt = body.Text
			if len(body.Skills) != 1 || body.Skills[0].ID != "lessmess-scaffold" {
				t.Error("scaffold skill not attached")
			}
			sent.Add(1)
			w.Write([]byte(`{"data":{}}`))
		case strings.HasSuffix(r.URL.Path, "/synthetic"):
			var body struct{ Text string }
			json.NewDecoder(r.Body).Decode(&body)
			synthetic = body.Text
			w.Write([]byte(`{"data":{"id":"msg_binding"}}`))
		case r.Method == "PATCH":
			w.WriteHeader(204)
		default:
			w.Write([]byte(`{"data":{"id":"ses_promote","title":"Chat title","time":{"updated":10}}}`))
		}
	})
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_promote", Title: "Chat title", Kind: "chat"})
	body := `{"title":"A real change","prefix":"ARC","confirmation":{"sessionID":"ses_promote","updated":10}}`
	for i := 0; i < 2; i++ {
		w := do(t, s.Handler(), "POST", "/api/sessions/ses_promote/promote", body)
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if sent.Load() != 1 || !strings.Contains(prompt, "does NOT authorize implementation") || !strings.Contains(prompt, "already agreed") {
		t.Fatal(sent.Load(), prompt)
	}
	// Admission alone never scaffolds or loses the conversation.
	if owner, _, _ := s.sessions.entry("ses_promote"); owner != unassignedKey {
		t.Fatal(owner)
	}
	w := do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"A real change","prefix":"ARC","session":"ses_promote"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	owner, e, _ := s.sessions.entry("ses_promote")
	if owner != resp["change"] || e.TitleState != "change" || e.Title != owner+" — A real change" {
		t.Fatal(owner, e)
	}
	if !strings.Contains(synthetic, "execution assistant for change "+owner) || !strings.Contains(resp["instructions"], "planning only") {
		t.Fatal("missing binding instructions", synthetic)
	}
	if len(s.sessions.listUnassigned()) != 0 {
		t.Fatal("promoted chat still standalone")
	}
	if w := do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"Another","prefix":"ARC","session":"ses_promote"}`); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if w := do(t, s.Handler(), "POST", "/api/sessions/ses_promote/promote", body); w.Code != 409 {
		t.Fatal(w.Code)
	}
}

func TestPromoteChatGuards(t *testing.T) {
	for _, tc := range []struct {
		name, kind, owner, body string
		busy                    bool
		want                    int
	}{
		{name: "unknown", want: 404},
		{name: "helper", kind: "helper", owner: unassignedKey, want: 404},
		{name: "bound", kind: "chat", owner: "2026-09-10-0", want: 409},
		{name: "busy", kind: "chat", owner: unassignedKey, busy: true, want: 409},
		{name: "stale", kind: "chat", owner: unassignedKey, body: `{"title":"x","prefix":"OK","confirmation":{"sessionID":"ses_guard","updated":9}}`, want: 409},
		{name: "invalid", kind: "chat", owner: unassignedKey, body: `{"title":"x","prefix":"longer"}`, want: 422},
		{name: "confirmation", kind: "chat", owner: unassignedKey, body: `{"title":"x","prefix":"OK"}`, want: 428},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/session/active" {
					if tc.busy {
						w.Write([]byte(`{"data":{"ses_guard":{"type":"running"}}}`))
					} else {
						w.Write([]byte(`{"data":{}}`))
					}
					return
				}
				if r.Method == "POST" {
					t.Error("guard sent mutation")
				}
				w.Write([]byte(`{"data":{"id":"ses_guard","time":{"updated":10}}}`))
			})
			if tc.owner != "" {
				_ = s.sessions.add(tc.owner, SessionEntry{Session: "ses_guard", Title: "x", Kind: tc.kind})
			}
			body := tc.body
			if body == "" {
				body = `{"title":"x","prefix":"OK","confirmation":{"sessionID":"ses_guard","updated":10}}`
			}
			w := do(t, s.Handler(), "POST", "/api/sessions/ses_guard/promote", body)
			if w.Code != tc.want {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
}

func TestConcurrentScaffoldCreatesOnlyOneChange(t *testing.T) {
	s := mappingServer(t, nil)
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_once", Title: "once"})
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			results <- do(t, s.Handler(), "POST", "/changes/scaffold", `{"title":"Once","prefix":"ON","session":"ses_once"}`).Code
		}()
	}
	a, b := <-results, <-results
	if a+b != 610 || a == b {
		t.Fatal(a, b)
	}
	if len(s.sessions.listUnassigned()) != 0 {
		t.Fatal("binding lost")
	}
}

func TestFailedPromotionPreservesChat(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session/active" {
			w.Write([]byte(`{"data":{}}`))
			return
		}
		if r.Method == "POST" {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"data":{"id":"ses_fail","time":{"updated":10}}}`))
	})
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_fail", Title: "Still a chat", Kind: "chat"})
	w := do(t, s.Handler(), "POST", "/api/sessions/ses_fail/promote", `{"title":"x","prefix":"OK","confirmation":{"sessionID":"ses_fail","updated":10}}`)
	owner, e, found := s.sessions.entry("ses_fail")
	if w.Code != 502 || !found || owner != unassignedKey || e.Promotion != 0 {
		t.Fatal(w.Code, owner, e)
	}
}

func TestScaffoldMappingFailureDoesNotCreateAgain(t *testing.T) {
	s := mappingServer(t, nil)
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_disk", Title: "before"})
	block := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(block, []byte("block"), 0600)
	s.sessions.path = filepath.Join(block, "sessions.json")
	body := `{"title":"Recover this change","prefix":"RC","session":"ses_disk"}`
	w := do(t, s.Handler(), "POST", "/changes/scaffold", body)
	if w.Code != 500 {
		t.Fatal(w.Code, w.Body)
	}
	var first map[string]string
	json.Unmarshal(w.Body.Bytes(), &first)
	w = do(t, s.Handler(), "POST", "/changes/scaffold", body)
	var retry map[string]string
	json.Unmarshal(w.Body.Bytes(), &retry)
	if w.Code != 409 || first["change"] == "" || retry["change"] != first["change"] {
		t.Fatal(w.Code, first, retry)
	}
	owner, e, _ := s.sessions.entry("ses_disk")
	if owner != unassignedKey || e.Title != "before" {
		t.Fatal("partial failed binding", owner, e)
	}
}
