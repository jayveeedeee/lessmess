package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"lessmess/internal/model"
)

// closeChange handles POST /changes/{id}/close: the user closes the change.
func (s *Server) closeChange(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.st.SetChangeStatus(id, model.OverallDone); err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("change closed", "id", id)
	if s.effectiveSettings().Docs.AutoGardenerOnClose {
		s.enqueueDocsRefresh(id) // best-effort; logs its own errors
	} else {
		slog.Info("docs auto-gardener disabled by settings; skipping refresh", "id", id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "status": string(model.OverallDone)})
}

// commitPrompt builds the message sent to a commit session.
func commitPrompt(changeID string) string {
	return fmt.Sprintf(`Create a git commit for this repository's current uncommitted changes.

1. Run git status and git diff (staged and unstaged) to review what is uncommitted.
2. Write a commit message following good practice: a concise summary line, then a body explaining the what and why. The change record for this work (if any) is changes/%[1]s/ — use it for context, not necessarily as the subject.
3. Stage everything relevant with git add -A (this respects .gitignore) and create the commit with that message.
4. NEVER push, amend, rebase, reset, or switch branches. Commit only.

Report the resulting commit hash and summary when done.`, changeID)
}

// commitChange handles POST /changes/{id}/commit: create + prime a commit
// session and map it to the change.
func (s *Server) commitChange(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.st.Change(id); err != nil {
		writeErr(w, err)
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	sess, err := s.spawnSession(ctx, id+" — git commit")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: sess.Title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.add(id, entry); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	if err := s.oc.Prompt(ctx, sess.ID, s.promptWith(commitPrompt(id), "commit")); err != nil {
		slog.Warn("commit prime failed", "session", sess.ID, "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"session": sess.ID, "error": "session created, but priming failed: " + err.Error()})
		return
	}
	slog.Info("commit session created", "change", id, "session", sess.ID)
	writeJSON(w, http.StatusCreated, map[string]string{"session": sess.ID})
}

// commitStatus handles GET /changes/{id}/commit-status?session={sid}: report
// whether the given session is still busy. A short server-side wait maps
// "returns quickly" to done and "times out" to busy.
func (s *Server) commitStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if !strings.HasPrefix(sessionID, "ses_") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ses_ session id required"})
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	err := s.oc.WaitDone(ctx, sessionID)
	done := err == nil
	writeJSON(w, http.StatusOK, map[string]bool{"done": done})
}

// reopenChange handles POST /changes/{id}/reopen: the user reopens a closed change.
func (s *Server) reopenChange(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.st.SetChangeStatus(id, model.OverallInProgress); err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("change reopened", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "status": string(model.OverallInProgress)})
}
