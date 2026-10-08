package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestChatNamesFirstAcceptedInput(t *testing.T) {
	var generations atomic.Int32
	var mu sync.Mutex
	title := "Codebase chat"
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/experimental/generate":{"post":{}},"/api/session/{sessionID}":{"patch":{}}}}`))
		case r.URL.Path == "/api/experimental/generate":
			generations.Add(1)
			w.Write([]byte(`{"data":{"text":"**Improve chat navigation**"}}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			w.Write([]byte(`{"data":{}}`))
		case r.Method == "PATCH":
			var body struct{ Title string }
			json.NewDecoder(r.Body).Decode(&body)
			title = body.Title
			w.WriteHeader(204)
		default:
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ses_name", "title": title}})
		}
	})
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_name", Title: title, Kind: "chat", TitleState: "eligible"})
	if generations.Load() != 0 {
		t.Fatal("prime triggered naming")
	}
	if w := do(t, s.Handler(), "POST", "/api/sessions/ses_name/chat/prompt", `{"text":""}`); w.Code != 422 {
		t.Fatal(w.Code)
	}
	_, e, _ := s.sessions.entry("ses_name")
	if e.TitleState != "eligible" {
		t.Fatal("rejected input named chat")
	}
	if w := do(t, s.Handler(), "POST", "/api/sessions/ses_name/chat/prompt", `{"text":"Improve the chat navigation"}`); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	s.chatNaming.Wait()
	_, e, _ = s.sessions.entry("ses_name")
	if e.Title != "Improve chat navigation" || e.TitleState != "auto" {
		t.Fatal(e)
	}
	s.afterChatInput("ses_name", "another topic", nil)
	s.chatNaming.Wait()
	if generations.Load() != 1 {
		t.Fatal("repeated generation")
	}
}

func TestChatNameFallbackAndOwnership(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_file", Title: "Codebase chat", Kind: "chat", TitleState: "eligible"})
	s.afterChatInput("ses_file", "", []string{"design.png"})
	s.chatNaming.Wait()
	_, e, _ := s.sessions.entry("ses_file")
	if e.Title != "Discuss design.png" || e.TitleState != "auto" {
		t.Fatal(e)
	}
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_custom", Title: "Keep me", Kind: "discussion"})
	s.afterChatInput("ses_custom", "replace me", nil)
	_, e, _ = s.sessions.entry("ses_custom")
	if e.Title != "Keep me" {
		t.Fatal(e)
	}
	if title := shortChatTitle(strings.Repeat("😀", 200)); len(title) > 200 {
		t.Fatal("oversized title")
	}
}

func TestChatNameQueuedAdmissionAndRejectedSend(t *testing.T) {
	var reject atomic.Bool
	reject.Store(true)
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/prompt":{"post":{"properties":{"id":{},"delivery":{}}}}}}`))
		case strings.HasSuffix(r.URL.Path, "/message"):
			w.Write([]byte(`{"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "/prompt"):
			if reject.Load() {
				w.WriteHeader(503)
				return
			}
			w.Write([]byte(`{"data":{"id":"msg_first","sessionID":"ses_queued","type":"user","delivery":"queue"}}`))
		default:
			w.Write([]byte(`{"data":{"id":"ses_queued","title":"Codebase chat"}}`))
		}
	})
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_queued", Title: "Codebase chat", Kind: "chat", TitleState: "eligible"})
	body := `{"id":"msg_first","text":"Queue design","delivery":"queue"}`
	if w := do(t, s.Handler(), "POST", "/api/sessions/ses_queued/deliver", body); w.Code != 502 {
		t.Fatal(w.Code, w.Body)
	}
	_, e, _ := s.sessions.entry("ses_queued")
	if e.TitleState != "eligible" {
		t.Fatal("rejected input named chat", e)
	}
	reject.Store(false)
	if w := do(t, s.Handler(), "POST", "/api/sessions/ses_queued/deliver", body); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	s.chatNaming.Wait()
	_, e, _ = s.sessions.entry("ses_queued")
	if e.Title != "Queue design" || e.TitleState != "auto" {
		t.Fatal(e)
	}
}

func TestLateChatNamingDoesNotOverwriteOwnership(t *testing.T) {
	for _, action := range []string{"manual", "change", "unlink"} {
		t.Run(action, func(t *testing.T) {
			started, finish := make(chan struct{}), make(chan struct{})
			s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/openapi.json":
					w.Write([]byte(`{"paths":{"/api/experimental/generate":{"post":{}}}}`))
				case "/api/experimental/generate":
					close(started)
					<-finish
					w.Write([]byte(`{"data":{"text":"Too late"}}`))
				default:
					w.Write([]byte(`{"data":{"id":"ses_late","title":"Codebase chat"}}`))
				}
			})
			_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_late", Title: "Codebase chat", Kind: "chat", TitleState: "eligible"})
			s.afterChatInput("ses_late", "initial", nil)
			<-started
			switch action {
			case "manual":
				_ = s.sessions.updateTitle("ses_late", "My title")
			case "change":
				_, _ = s.sessions.moveToChange("ses_late", "bound")
				_ = s.sessions.updateTitle("ses_late", "Change title")
			case "unlink":
				_, _ = s.sessions.remove(unassignedKey, "ses_late")
			}
			close(finish)
			s.chatNaming.Wait()
			_, e, found := s.sessions.entry("ses_late")
			if action == "unlink" && found {
				t.Fatal("resurrected mapping")
			}
			if found && (e.Title == "Too late" || e.TitleState == "auto") {
				t.Fatal(e)
			}
		})
	}
}
