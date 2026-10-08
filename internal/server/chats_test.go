package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatsLegacyAndKinds(t *testing.T) {
	s := mappingServer(t, nil)
	s.Base = "/p/demo"
	for _, e := range []SessionEntry{
		{Session: "ses_old", Title: "legacy", Created: "2026-01-01T00:00:00Z"},
		{Session: "ses_chat", Title: "chat", Kind: "chat", Updated: "2026-10-08T12:00:00Z"},
		{Session: "ses_helper", Title: "commit", Kind: "helper"},
		{Session: "ses_commit", Title: "repo — git commit"},
		{Session: "ses_child", Title: "child", Kind: "helper", Parent: "ses_chat"},
		{Session: "ses_fork", Title: "fork", Kind: "fork", Parent: "ses_chat"},
		{Session: "ses_oldfork", Title: "old fork", Parent: "ses_old"},
	} {
		if err := s.sessions.addUnassigned(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_bound", Title: "bound"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "GET", "/api/chats", "")
	var body struct {
		Chats []chatSummary `json:"chats"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(body.Chats) != 4 || body.Chats[0].Session != "ses_chat" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if body.Chats[0].URL != "/p/demo/chats?session=ses_chat" {
		t.Fatal(body.Chats[0])
	}
	loaded, err := loadMapping(s.sessions.path)
	if err != nil || len(loaded.listUnassigned()) != 7 {
		t.Fatalf("reload: %v", err)
	}
}

func TestChatsLiveActivityAndFallback(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "ses_live") {
			w.Write([]byte(`{"data":{"id":"ses_live","title":"live title","time":{"updated":1791453600000}}}`))
			return
		}
		w.WriteHeader(503)
	})
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_live", Title: "old"})
	_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_down", Title: "fallback", Created: "2026-01-01T00:00:00Z"})
	w := do(t, s.Handler(), "GET", "/api/chats", "")
	if !strings.Contains(w.Body.String(), `"title":"live title"`) || !strings.Contains(w.Body.String(), `"title":"fallback"`) {
		t.Fatal(w.Body)
	}
	_, e, _ := s.sessions.entry("ses_live")
	if e.Title != "live title" || e.Updated == "" {
		t.Fatal(e)
	}
}

func TestChatMetadataSaveRollback(t *testing.T) {
	m, _ := loadMapping(filepath.Join(t.TempDir(), "sessions.json"))
	_ = m.addUnassigned(SessionEntry{Session: "ses_x", Title: "before", TitleState: "eligible"})
	block := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(block, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	m.path = filepath.Join(block, "sessions.json")
	if err := m.updateTitle("ses_x", "after"); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := m.moveToChange("ses_x", "change"); err == nil {
		t.Fatal("expected failure")
	}
	owner, e, _ := m.entry("ses_x")
	if owner != unassignedKey || e.Title != "before" || e.TitleState != "eligible" {
		t.Fatal(owner, e)
	}
	if len(m.list("change")) != 0 {
		t.Fatal("partial move")
	}
	if _, err := m.editEntry("ses_x", func(_ string, e *SessionEntry) bool { e.Updated = "after"; return true }); err == nil {
		t.Fatal("expected failure")
	}
	_, e, _ = m.entry("ses_x")
	if e.Updated != "" {
		t.Fatal("partial metadata update")
	}
}

func TestChatsPageAndNavigation(t *testing.T) {
	for _, base := range []string{"", "/p/demo"} {
		t.Run(base, func(t *testing.T) {
			s := mappingServer(t, nil)
			s.Base = base
			_ = s.sessions.addUnassigned(SessionEntry{Session: "ses_page", Title: "A <safe> title", Kind: "chat"})
			body := htmlGet(t, s.Handler(), "/chats?session=ses_page", false).Body.String()
			for _, want := range []string{`data-page="chats"`, `id="location-current">Chats`, `href="` + base + `/chats" class="active"`, `data-chat="ses_page"`, `href="` + base + `/chats?session=ses_page"`, "A &lt;safe&gt; title", "New chat", `id="detail"`} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			index := htmlGet(t, s.Handler(), "/", false).Body.String()
			if strings.Contains(index, "discussions-list") || strings.Contains(index, "<h2>Discussions") {
				t.Fatal("Discussions remain on Changes")
			}
			if !strings.Contains(index, "New change session") {
				t.Fatal("planning entry lost")
			}
			css := do(t, s.Handler(), "GET", "/static/app.css", "").Body.String()
			if !strings.Contains(css, `.section-head [data-new-chat] { margin-left: auto; }`) {
				t.Error("New chat must align to the right of its section header")
			}
		})
	}
}

func TestSectionLocalCreationAndSettingsIcon(t *testing.T) {
	for _, base := range []string{"", "/p/demo"} {
		t.Run(base, func(t *testing.T) {
			s := mappingServer(t, nil)
			s.Base = base
			for _, path := range []string{"/", "/chats", "/explorer", "/settings", "/settings/opencode", "/setup"} {
				t.Run(path, func(t *testing.T) {
					w := htmlGet(t, s.Handler(), path, false)
					if w.Code != http.StatusOK {
						t.Fatalf("page status = %d: %s", w.Code, w.Body)
					}
					body := w.Body.String()
					header, _, _ := strings.Cut(body, "</header>")
					wantChat := 0
					if path == "/chats" {
						wantChat = 1
					}
					if got := strings.Count(body, "data-new-chat"); got != wantChat {
						t.Errorf("New chat actions = %d, want %d", got, wantChat)
					}
					if strings.Contains(header, "data-new-chat") || strings.Contains(body, `id="chat-btn"`) {
						t.Error("shared header must not offer chat creation")
					}
					if got := strings.Contains(body, `hx-post="`+base+`/changes/session"`); got != (path == "/") {
						t.Error("New change session must appear only in Changes")
					}
					if path == "/setup" {
						if strings.Contains(header, "topnav-icon") {
							t.Error("setup must not show Settings navigation")
						}
						return
					}
					active := path == "/settings" || path == "/settings/opencode"
					class := "topnav-icon"
					if active {
						class += " active"
					}
					link := `<a href="` + base + `/settings" class="` + class + `" aria-label="Settings" title="Settings"`
					if active {
						link += ` aria-current="page"`
					}
					if !strings.Contains(header, link+`><svg`) || !strings.Contains(header, `<circle cx="12" cy="12" r="3"/>`) {
						t.Error("accessible Settings gear or active state missing")
					}
					if strings.Contains(header, ">Settings</a>") {
						t.Error("Settings navigation must use an icon rather than visible text")
					}
					if path == "/settings/opencode" && !strings.Contains(body, `id="oc-return-chat" class="btn-ghost" href="`+base+`/chats">Return to Chats</a>`) {
						t.Error("service management must link back to Chats without spawning")
					}
				})
			}
		})
	}
}
