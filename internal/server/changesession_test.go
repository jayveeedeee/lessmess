package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChangeSessionFlow(t *testing.T) {
	var prompted struct {
		session string
		text    string
	}
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_flow","title":"t","location":{"directory":"/x"}}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			prompted.session = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/session/"), "/prompt")
			prompted.text = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"Flow objective","prefix":"FL"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	changeID := resp["change"]
	if changeID == "" || resp["session"] != "ses_flow" {
		t.Fatalf("resp = %v", resp)
	}

	// Change scaffold exists and is reachable.
	if w := do(t, s.Handler(), "GET", "/changes/"+changeID, ""); w.Code != 200 {
		t.Fatalf("new board: code = %d", w.Code)
	}
	// Session mapped.
	entries := s.sessions.list(changeID)
	if len(entries) != 1 || entries[0].Session != "ses_flow" {
		t.Fatalf("mapping = %+v", entries)
	}
	// Prompt was sent to the right session and references the change.
	if prompted.session != "ses_flow" {
		t.Fatalf("prompted session = %q", prompted.session)
	}
	if !strings.Contains(prompted.text, changeID) || !strings.Contains(prompted.text, "Flow objective") || !strings.Contains(prompted.text, "AGENTS.md") {
		t.Fatalf("prompt text = %q", prompted.text)
	}
}

func TestChangeSessionHXRedirect(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/prompt") {
			w.Write([]byte(`{"data":{}}`))
			return
		}
		w.Write([]byte(`{"data":{"id":"ses_hx","title":"t","location":{"directory":"/x"}}}`))
	})
	r, _ := http.NewRequest("POST", "/changes/session", strings.NewReader("title=HX+thing&prefix=HX"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	redir := w.Header().Get("HX-Redirect")
	if !strings.HasPrefix(redir, "/changes/") || !strings.Contains(redir, "?session=ses_hx") {
		t.Fatalf("HX-Redirect = %q", redir)
	}
}

func TestChangeSessionNoService(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"x"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestChangeSessionCreateFailsKeepsChange(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"_tag":"UnknownError","message":"boom"}`))
	})
	w := do(t, s.Handler(), "POST", "/changes/session", `{"title":"Keepme"}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	// The change was created despite the session failure.
	if resp["change"] == "" {
		t.Fatalf("resp = %v", resp)
	}
	if w := do(t, s.Handler(), "GET", "/changes/"+resp["change"], ""); w.Code != 200 {
		t.Fatalf("change not kept: code = %d", w.Code)
	}
	if len(s.sessions.list(resp["change"])) != 0 {
		t.Fatal("no session should be mapped on failure")
	}
}

func TestChangeSessionPlaceholderTitle(t *testing.T) {
	var promptedText string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			w.Write([]byte(`{"data":{"id":"ses_ph","title":"t","location":{"directory":"/x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			promptedText = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	w := do(t, s.Handler(), "POST", "/changes/session", `{}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	changeID := resp["change"]
	if changeID == "" || resp["session"] != "ses_ph" {
		t.Fatalf("resp = %v", resp)
	}

	// Scaffold used a placeholder title and an empty prefix.
	root, err := s.st.Root()
	if err != nil {
		t.Fatal(err)
	}
	var row *struct{ Title, Prefix string }
	for _, r := range root.Rows {
		if r.Change == changeID {
			row = &struct{ Title, Prefix string }{r.Title, r.Prefix}
		}
	}
	if row == nil {
		t.Fatal("root row missing")
	}
	if !strings.HasPrefix(row.Title, "untitled-") {
		t.Fatalf("title = %q, want untitled-*", row.Title)
	}
	if row.Prefix != "—" {
		t.Fatalf("prefix = %q, want —", row.Prefix)
	}

	// Prompt carries the titling/prefix/rename instructions with the session ID.
	for _, want := range []string{"placeholder", "task-ID prefix", "opencode2 api post /api/session/ses_ph/rename", changeID} {
		if !strings.Contains(promptedText, want) {
			t.Errorf("prompt missing %q:\n%s", want, promptedText)
		}
	}
}

func TestPlaceholderTitleFormat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		ti := placeholderTitle()
		if !strings.HasPrefix(ti, "untitled-") {
			t.Fatalf("title = %q", ti)
		}
		seen[ti] = true
	}
	if len(seen) < 2 {
		t.Fatalf("placeholder titles not varying: %v", seen)
	}
}
