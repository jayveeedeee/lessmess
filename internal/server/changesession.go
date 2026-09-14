package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var prefixRe = regexp.MustCompile(`^[A-Z0-9]{2,4}$`)

// discussionPrompt builds the message for a pre-scaffold discussion session.
// The agent discusses the objective and, only with the user's explicit
// approval, fires the deterministic scaffold trigger with the agreed
// title/prefix. Step 0 covers the empty state: a bare prime (nothing
// appended below it) must not investigate the repository; it invites the
// request in one line and waits. The API base URL and the session's own ID
// are injected.
func discussionPrompt(apiBase, sessionID string) string {
	return fmt.Sprintf(`You are a planning assistant for a repository that uses the change-management workflow defined in AGENTS.md. This session exists to plan a NEW change from the user's own request.

0. Empty state: this prime message may arrive before the user has typed anything. If no user request accompanies this message (nothing below it), do NOT investigate the repository — do not read changes/, any ledger, or open changes — and do not summarize anything. Reply with a single short line inviting the request (for example: "What would you like to build?") and stop. Every other instruction in this message — above, below, or in an appended addendum — applies only from the user's first message onward.
1. Discuss with the user what they want to build: objective, context, scope, and design options. Ask questions; help them decide.
2. DO NOT modify the repository in any way — no change directories, no edits, no scaffolds. Discussion only.
3. When the user EXPLICITLY agrees to start the work, choose a concise change title and a 2–4 letter uppercase task-ID prefix, then scaffold the change by running exactly this (replacing <title> and <prefix>):

curl -s -X POST %[1]s/changes/scaffold -H 'Content-Type: application/json' -d '{"title":"<title>","prefix":"<prefix>","session":"%[2]s"}'

This call creates the change directory, registers your title and prefix, renames this session, and links it to the new change. Report the returned change ID to the user.
4. Then refine changes/<id>/plan.md and break the work into verifiable tasks per AGENTS.md (task files plus matching ledger rows), keeping the user in the loop before any implementation.`, apiBase, sessionID)
}

// changePrompt builds the prime message for a session bound to an existing
// change: everything requested in the conversation is work on that change,
// and scaffolding a new change from it is forbidden.
func changePrompt(changeID string) string {
	return fmt.Sprintf(`You are a change execution assistant for a repository that uses the change-management workflow defined in AGENTS.md. This session is permanently bound to change %[1]s.

1. Read changes/%[1]s/plan.md and changes/%[1]s/ledger.md first — they hold the authoritative scope, design, and task status for this change.
2. Everything the user asks for in this conversation is work on THIS change: refine changes/%[1]s/plan.md, add or update task files and ledger rows under its existing task-ID prefix, and keep ledger statuses current per AGENTS.md.
3. NEVER create a new change directory and NEVER call the /changes/scaffold endpoint. If the user asks for genuinely unrelated work, explain that it belongs in a separate change and ask them to start a new discussion from the index page.`, changeID)
}

// apiBase returns the lessmess base URL used in agent-facing prompts.
func (s *Server) apiBase() string {
	if s.PublicBase != "" {
		return "http://" + s.PublicBase
	}
	return "http://127.0.0.1:8080"
}

// createDiscussionSession handles POST /changes/session: create a
// discussion-only opencode session (no repository writes) and map it to the
// unassigned bucket.
func (s *Server) createDiscussionSession(w http.ResponseWriter, r *http.Request) {
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	var req createSessionRequest
	if isJSON(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		_ = r.ParseForm()
		req.Title = r.FormValue("title")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New change discussion"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	sess, err := s.spawnSession(ctx, title)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	if err := s.oc.Prompt(ctx, sess.ID, s.promptWith(discussionPrompt(s.apiBase(), sess.ID), "discussion")); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "prime discussion session: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.addUnassigned(entry); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("discussion session created", "session", sess.ID)
	writeJSON(w, http.StatusCreated, sessionResponse{Session: entry.Session, Title: entry.Title, Created: entry.Created, Live: true})
}

// scaffoldRequest is the body of POST /changes/scaffold, the deterministic
// trigger the discussion agent calls after explicit user approval.
type scaffoldRequest struct {
	Title   string `json:"title"`
	Prefix  string `json:"prefix"`
	Session string `json:"session"`
}

// scaffoldChange handles POST /changes/scaffold: build the change with the
// agreed title/prefix, rename the session, and move it to the change.
func (s *Server) scaffoldChange(w http.ResponseWriter, r *http.Request) {
	var req scaffoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || strings.Contains(req.Title, "|") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "title must be non-empty and contain no |"})
		return
	}
	if req.Prefix != "" && !prefixRe.MatchString(req.Prefix) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "prefix must be 2–4 uppercase letters/digits or empty"})
		return
	}
	if !strings.HasPrefix(req.Session, "ses_") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "valid ses_ session id required"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}

	// A session already bound to a change can never scaffold a new one: all
	// of its work belongs to that change. The message is agent-facing so a
	// misled agent self-corrects.
	if bound, ok := s.sessions.changeOf(req.Session); ok {
		slog.Warn("scaffold refused: session already bound", "session", req.Session, "change", bound)
		writeJSON(w, http.StatusConflict, map[string]string{
			"change": bound,
			"error":  "session already bound to change " + bound + "; continue the work within that change (refine plan.md, add task files and ledger rows) — do not scaffold a new change",
		})
		return
	}

	id, err := s.st.CreateChange(req.Title, req.Prefix, s.effectiveSettings().Git.DefaultBranch, time.Now().Format("2006-01-02"))
	if err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("change scaffolded via API trigger", "id", id, "title", req.Title, "prefix", req.Prefix, "session", req.Session)

	// Rename the session server-side (best effort) and move the mapping.
	if s.oc != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		if err := s.oc.RenameSession(ctx, req.Session, id+" — "+req.Title); err != nil {
			slog.Warn("session rename failed", "session", req.Session, "err", err)
		}
		cancel()
	}
	moved, err := s.sessions.moveToChange(req.Session, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"change": id, "error": "persist mapping: " + err.Error()})
		return
	}
	if !moved {
		// Idempotent for retries; also covers sessions created outside the flow.
		slog.Warn("session not in unassigned bucket; linking directly", "session", req.Session, "change", id)
		if s.oc != nil {
			title := id + " — " + req.Title
			_ = s.sessions.add(id, SessionEntry{Session: req.Session, Title: title, Created: time.Now().Format(time.RFC3339)})
		}
	}
	writeJSON(w, http.StatusCreated, map[string]string{"change": id, "session": req.Session, "title": req.Title})
}
