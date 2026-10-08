package server

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Empty kinds are deliberately visible: old mapping files must not lose chats.
func standaloneChat(e SessionEntry) bool {
	if e.Kind == "helper" || (e.Parent != "" && e.Kind != "" && e.Kind != "fork") {
		return false
	}
	return e.Kind != "" || e.Title != "repo — git commit"
}

type chatSummary struct {
	Session string `json:"session"`
	Title   string `json:"title"`
	Updated string `json:"updated"`
	Kind    string `json:"kind,omitempty"`
	URL     string `json:"url"`
	Live    bool   `json:"live"`
}

// editEntry provides an atomic, rollback-safe update to optional personal metadata.
func (m *mapping) editEntry(session string, edit func(string, *SessionEntry) bool) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for owner, entries := range m.data {
		for i := range entries {
			if entries[i].Session != session {
				continue
			}
			old := entries[i]
			if !edit(owner, &entries[i]) {
				return false, nil
			}
			if err := m.save(); err != nil {
				entries[i] = old
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) chatSummaries(r *http.Request) []chatSummary {
	entries := s.sessions.listUnassigned()
	out := make([]chatSummary, 0, len(entries))
	for _, e := range entries {
		if standaloneChat(e) {
			updated := e.Updated
			if updated == "" {
				updated = e.Created
			}
			out = append(out, chatSummary{Session: e.Session, Title: e.Title, Updated: updated, Kind: e.Kind, URL: s.Base + "/chats?session=" + url.QueryEscape(e.Session)})
		}
	}
	if s.oc != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		// One shared deadline and bounded concurrency, not N sequential timeouts.
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for i := range out {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				defer func() { <-sem }()
				live, err := s.oc.GetSession(ctx, out[i].Session)
				if err != nil {
					return
				}
				out[i].Live = true
				_, current, known := s.sessions.entry(out[i].Session)
				owned := known && current.TitleState != "" && current.TitleState != "eligible"
				if owned {
					out[i].Title = current.Title
				} else if strings.TrimSpace(live.Title) != "" {
					out[i].Title = live.Title
				}
				if live.Time.Updated > 0 {
					stored, _ := time.Parse(time.RFC3339Nano, out[i].Updated)
					if at := time.UnixMilli(live.Time.Updated).UTC(); at.After(stored) {
						out[i].Updated = at.Format(time.RFC3339Nano)
					}
				}
				_, _ = s.sessions.editEntry(out[i].Session, func(owner string, e *SessionEntry) bool {
					if owner != unassignedKey {
						return false
					}
					if e.Updated == out[i].Updated && e.Title == out[i].Title {
						return false
					}
					// Do not overwrite a title owned by an in-flight naming operation.
					if e.TitleState == "" {
						e.Title = out[i].Title
					}
					stored, _ := time.Parse(time.RFC3339Nano, e.Updated)
					at, _ := time.Parse(time.RFC3339Nano, out[i].Updated)
					if at.After(stored) {
						e.Updated = out[i].Updated
					}
					return true
				})
			}(i)
		}
		wg.Wait()
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, out[i].Updated)
		b, _ := time.Parse(time.RFC3339Nano, out[j].Updated)
		if !a.Equal(b) {
			return a.After(b)
		}
		return out[i].Session < out[j].Session
	})
	return out
}

func (s *Server) listChats(w http.ResponseWriter, r *http.Request) {
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": s.chatSummaries(r)})
}

func (s *Server) chatsPage(w http.ResponseWriter, r *http.Request) {
	view := struct {
		Chats []chatSummary
		Error string
	}{}
	if s.mapErr != nil {
		view.Error = "Chat history is unavailable: session mapping unreadable."
	} else {
		view.Chats = s.chatSummaries(r)
	}
	s.rend.render(w, s.rend.chats, "layout", pageData{Title: "Chats", Page: "chats", Data: view})
}
