package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lessmess/internal/opencode"
)

// baseRequest builds a request with explicit headers (do() pins JSON
// acceptance, which is the wrong shape for the HTML/htmx branches).
func baseRequest(method, path, body string, headers map[string]string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func baseServe(t *testing.T, h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// Base-path plumbing: with a server mounted under /p/demo, every
// server-emitted URL carries the prefix. The empty-base shape is pinned
// byte-for-byte by the rest of the suite (e.g.
// TestFormEncodedCreateChangeRedirect), so legacy mode cannot drift.
func TestServerBasePathEmissions(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	s.Base = "/p/demo"
	h := s.Handler()

	id := "2026-09-10-0"

	// HTML board requests redirect into the chat-first flow, prefixed.
	w := baseServe(t, h, baseRequest("GET", "/changes/"+id, "", map[string]string{"Accept": "text/html"}))
	if w.Code != http.StatusFound || !strings.HasPrefix(w.Header().Get("Location"), "/p/demo/?change=") {
		t.Fatalf("redirect = %d %q, want prefixed /p/demo/?change=", w.Code, w.Header().Get("Location"))
	}
	// ?session= survives the prefix.
	w = baseServe(t, h, baseRequest("GET", "/changes/"+id+"?session=ses1", "", map[string]string{"Accept": "text/html"}))
	if loc := w.Header().Get("Location"); !strings.HasPrefix(loc, "/p/demo/?change=") || !strings.Contains(loc, "session=ses1") {
		t.Fatalf("redirect with session = %q", loc)
	}

	// htmx change creation redirects through the prefix.
	w = baseServe(t, h, baseRequest("POST", "/changes/", "title=New+thing&prefix=NT", map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"HX-Request":   "true",
	}))
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	if redir := w.Header().Get("HX-Redirect"); !strings.HasPrefix(redir, "/p/demo/?change=") {
		t.Fatalf("HX-Redirect = %q, want /p/demo/?change=…", redir)
	}

	// The chat Work panel's task feed links are prefixed.
	w = do(t, h, "GET", "/changes/"+id+"/tasks", "")
	if w.Code != http.StatusOK {
		t.Fatalf("tasks feed = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "/p/demo/changes/"+id+"/tasks/00-first.md") {
		t.Error("tasks feed missing prefixed task href")
	}
}

// Chat transcript tool/file URLs are server-emitted; the base flows into
// the view builder directly.
func TestChatViewBasePathEmissions(t *testing.T) {
	msg := opencode.Message{
		ID:   "msg1",
		Type: "assistant",
		Content: []opencode.MessagePart{
			opencode.ToolPart{Type: "tool", ID: "tool1", Name: "bash", State: opencode.ToolState{Status: "completed"}},
		},
	}
	view := makeChatMessageView("/p/demo", "ses_chat", msg)
	if len(view.Parts) == 0 || view.Parts[0].ToolURL != "/p/demo/api/sessions/ses_chat/chat/messages/msg1/tools/tool1" {
		t.Fatalf("tool URL = %+v, want prefixed path", view.Parts)
	}
	legacy := makeChatMessageView("", "ses_chat", msg)
	if legacy.Parts[0].ToolURL != "/api/sessions/ses_chat/chat/messages/msg1/tools/tool1" {
		t.Fatalf("empty-base tool URL = %q", legacy.Parts[0].ToolURL)
	}
}
