package server

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

func (s *Server) promoteChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionID")
	if !s.lifecycleReady(w, id) {
		return
	}
	if s.mapErr != nil {
		writeJSON(w, 503, map[string]string{"error": "session mapping unreadable"})
		return
	}
	var req struct {
		Title        string                `json:"title"`
		Prefix       string                `json:"prefix"`
		Confirmation lifecycleConfirmation `json:"confirmation"`
	}
	if !decodeChatJSON(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > 256 || strings.Contains(req.Title, "|") || strings.IndexFunc(req.Title, unicode.IsControl) >= 0 || !prefixRe.MatchString(req.Prefix) {
		writeJSON(w, 422, map[string]string{"error": "a title (1–256 characters, no pipes or control characters) and 2–4 uppercase letter/digit prefix are required"})
		return
	}
	s.sessionOps.Lock()
	defer s.sessionOps.Unlock()
	owner, entry, found := s.sessions.entry(id)
	if !found || !standaloneChat(entry) {
		writeJSON(w, 404, map[string]string{"error": "standalone chat not found in this project"})
		return
	}
	if owner != unassignedKey {
		writeJSON(w, 409, map[string]string{"error": "this chat already belongs to a change", "change": owner})
		return
	}
	if req.Confirmation.SessionID != id || req.Confirmation.Updated == 0 {
		writeJSON(w, 428, map[string]string{"error": "confirm this session and its current revision"})
		return
	}
	// A repeated admission of this exact confirmation never sends another input.
	if entry.Promotion == req.Confirmation.Updated {
		writeJSON(w, 202, map[string]bool{"ok": true})
		return
	}
	if _, ok := s.confirmedIdle(w, r, id, req.Confirmation); !ok {
		return
	}
	admitted, err := s.sessions.editEntry(id, func(owner string, e *SessionEntry) bool {
		if owner != unassignedKey {
			return false
		}
		e.Promotion = req.Confirmation.Updated
		return true
	})
	if err != nil || !admitted {
		writeJSON(w, 500, map[string]string{"error": "promotion could not be recorded; no instruction was sent"})
		return
	}
	prompt := fmt.Sprintf("The user confirmed Promote to change. They explicitly approve scaffolding and planning a tracked change from this conversation, with title %q and prefix %q. Load the lessmess-scaffold skill and use its existing scaffold procedure with your session ID %s. The title and prefix are already agreed; preserve this session and its context. Populate the plan and tasks from the discussion, then validate. This approval does NOT authorize implementation or unrelated code edits; wait for a separate explicit go-ahead after planning.", req.Title, req.Prefix, id)
	if err := s.oc.PromptWithFilesAndSkills(r.Context(), id, prompt, nil, []string{"lessmess-scaffold"}); err != nil {
		_, _ = s.sessions.editEntry(id, func(_ string, e *SessionEntry) bool { e.Promotion = 0; return true })
		writeChatUpstreamError(w, err)
		return
	}
	writeJSON(w, 202, map[string]bool{"ok": true})
}
