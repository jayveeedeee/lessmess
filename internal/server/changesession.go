package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var (
	placeholderAdjectives = []string{"amber", "brisk", "crimson", "dusky", "ember", "faint", "golden", "hollow", "ivory", "jade", "keen", "lunar", "misty", "nimble", "onyx", "pale", "quiet", "rusty", "silent", "teal", "umber", "velvet", "wild", "zephyr"}
	placeholderNouns      = []string{"adder", "basin", "cedar", "delta", "elm", "falcon", "grove", "heron", "isle", "juniper", "kite", "lark", "mesa", "newt", "otter", "pine", "quail", "ridge", "sparrow", "tern", "urchin", "viper", "wren", "yew"}
)

// placeholderTitle returns a random stand-in title the agent will replace.
func placeholderTitle() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("untitled-%s-%s",
		placeholderAdjectives[r.Intn(len(placeholderAdjectives))],
		placeholderNouns[r.Intn(len(placeholderNouns))])
}

// primePrompt builds the first message sent to a freshly scaffolded
// change session. The agent assigns the real title/prefix/session name
// once it knows the objective.
func primePrompt(changeID, title, sessionID string) string {
	return fmt.Sprintf(`You are starting a new change in this repository: change %[1]s (currently a placeholder titled %[2]q).

The user will tell you the objective in this conversation. Once you know it, do all of the following:

1. Choose a concise change title and apply it in changes/%[1]s/plan.md (document title and H1) and in the root ledger changes/ledger.md (this change's Title cell).
2. Choose a 2–4 letter uppercase task-ID prefix and register it in the root ledger's ID prefix cell for this change (replace the —).
3. Rename this opencode session to "%[1]s — <title>" by running: opencode2 api post /api/session/%[3]s/rename --data '{"title":"<title>"}'
4. Refine changes/%[1]s/plan.md (objective, context, scope, design decisions, acceptance criteria) and break it into verifiable tasks under changes/%[1]s/tasks/ with matching rows in changes/%[1]s/ledger.md, following AGENTS.md exactly.
5. Present the refined plan to the user before implementing anything.`, changeID, title, sessionID)
}

type changeSessionRequest struct {
	Title  string `json:"title"`
	Prefix string `json:"prefix"`
}

// createChangeWithSession handles POST /changes/session: scaffold a change,
// create + prime an opencode session, link it, and redirect to the board.
func (s *Server) createChangeWithSession(w http.ResponseWriter, r *http.Request) {
	var req changeSessionRequest
	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad form body"})
			return
		}
		req.Title = r.FormValue("title")
		req.Prefix = r.FormValue("prefix")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = placeholderTitle()
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}

	id, err := s.st.CreateChange(title, req.Prefix, time.Now().Format("2006-01-02"))
	if err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("create change (session flow)", "id", id, "title", title)

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	sess, err := s.oc.CreateSession(ctx, id+" — "+title, s.st.Dir)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"change": id, "error": "change created, but session failed: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: sess.Title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.add(id, entry); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"change": id, "error": "persist mapping: " + err.Error()})
		return
	}
	if err := s.oc.Prompt(ctx, sess.ID, primePrompt(id, title, sess.ID)); err != nil {
		slog.Warn("prime prompt failed", "session", sess.ID, "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"change": id, "session": sess.ID, "error": "session created, but priming failed: " + err.Error()})
		return
	}
	slog.Info("session created and primed", "change", id, "session", sess.ID)

	if isHX(r) {
		w.Header().Set("HX-Redirect", "/changes/"+id+"?session="+sess.ID)
		w.WriteHeader(http.StatusOK)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"change": id, "session": sess.ID})
}
