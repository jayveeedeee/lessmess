package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// Per-setting Change button: POST /api/settings/change starts (or reuses)
// a discussion session that runs the normal change flow, primed with
// exactly which setting was clicked. One settings-initiated discussion is
// active at a time — tracked in .lessmess/settings-change.json — and it is
// reused until its change is explicitly closed (Done/Cancelled).

const settingsChangeStateFile = "settings-change.json"

// settingsChangeState is the gitignored state file holding the active
// settings-discussion session id.
type settingsChangeState struct {
	Session string `json:"session"`
}

func settingsChangeStatePath(repoDir string) string {
	return filepath.Join(repoDir, store.StateDirName, settingsChangeStateFile)
}

// loadSettingsChangeState reads the state file, tolerating missing and
// corrupt files as "no active discussion".
func loadSettingsChangeState(repoDir string) settingsChangeState {
	var st settingsChangeState
	b, err := os.ReadFile(settingsChangeStatePath(repoDir))
	if err != nil || json.Unmarshal(b, &st) != nil {
		return settingsChangeState{}
	}
	return st
}

func saveSettingsChangeState(repoDir string, st settingsChangeState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	p := settingsChangeStatePath(repoDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return model.WriteFileAtomic(p, append(b, '\n'), 0o644)
}

// settingsChangeRequest is the body of POST /api/settings/change. The
// server computes value and source itself; the client only reports which
// field and which scope the page was editing.
type settingsChangeRequest struct {
	Field string `json:"field"`
	Scope string `json:"scope"`
}

// settingsFieldValue extracts one effective value by dotted path.
func settingsFieldValue(e EffectiveSettings, field string) (string, bool) {
	switch field {
	case "general.projectName":
		return e.General.ProjectName, true
	case "session.agent":
		return e.Session.Agent, true
	case "session.model":
		return e.Session.Model, true
	case "prompts.discussion":
		return e.Prompts.Discussion, true
	case "prompts.change":
		return e.Prompts.Change, true
	case "prompts.commit":
		return e.Prompts.Commit, true
	case "prompts.repoCommit":
		return e.Prompts.RepoCommit, true
	case "prompts.gardener":
		return e.Prompts.Gardener, true
	case "prompts.explorer":
		return e.Prompts.Explorer, true
	case "git.defaultBranch":
		return e.Git.DefaultBranch, true
	case "git.worktrees":
		return strconv.FormatBool(e.Git.Worktrees), true
	case "git.reviewModel":
		return e.Git.ReviewModel, true
	case "ui.showArchived":
		return strconv.FormatBool(e.UI.ShowArchived), true
	case "ui.accent":
		return e.UI.Accent, true
	case "docs.autoGardenerOnClose":
		return strconv.FormatBool(e.Docs.AutoGardenerOnClose), true
	case "docs.gardenerModel":
		return e.Docs.GardenerModel, true
	}
	return "", false
}

// settingsChangeContext builds the agent-facing context block: appended to
// the fresh discussion prime, or sent as the fold-in message on reuse.
func settingsChangeContext(field, value, source, scope string, reuse bool) string {
	if value == "" {
		value = "(unset — built-in default)"
	}
	if len(value) > 160 {
		value = value[:157] + "..."
	}
	group := field
	if i := strings.IndexByte(field, '.'); i >= 0 {
		group = field[:i]
	}
	group = strings.ToUpper(group[:1]) + group[1:]
	detail := fmt.Sprintf(`- Setting: %[1]s (group: %[2]s)
- Current effective value: %[3]s (source: %[4]s)
- Settings page scope when clicked: %[5]s`, field, group, value, source, scope)
	if reuse {
		return fmt.Sprintf(`The user clicked Change for another setting on the Settings page:

%[1]s

Fold this into the current discussion. If it is clearly unrelated to the work in progress, say so and suggest how to handle it.`, detail)
	}
	return fmt.Sprintf(`The user opened this discussion from the Settings page by clicking the Change button for one specific setting:

%[1]s

Discuss what the user wants for this setting and follow the normal flow above. The settings system is documented in README.md ("Settings"): project settings live in lessmess.json (committed), personal overrides in .lessmess/settings.json (gitignored), and PUT /api/settings?scope=... writes one layer.`, detail)
}

// settingsChange handles POST /api/settings/change.
func (s *Server) settingsChange(w http.ResponseWriter, r *http.Request) {
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	var req settingsChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	req.Field = strings.TrimSpace(req.Field)
	if req.Scope != SettingsScopeProject && req.Scope != SettingsScopePersonal {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errSettingsBadScope.Error()})
		return
	}
	view := settingsAPIView(s.st.Dir)
	value, ok := settingsFieldValue(view.Effective, req.Field)
	if !ok {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "unknown setting field: " + req.Field})
		return
	}
	source := view.Sources[req.Field]

	// Reuse the active settings discussion while its change is open.
	if st := loadSettingsChangeState(s.st.Dir); st.Session != "" && s.settingsChangeReusable(st.Session) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := s.oc.Prompt(ctx, st.Session, settingsChangeContext(req.Field, value, source, req.Scope, true)); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "message active settings discussion: " + err.Error()})
			return
		}
		title := st.Session
		if sess, err := s.oc.GetSession(ctx, st.Session); err == nil {
			title = sess.Title
		}
		slog.Info("settings change folded into active discussion", "field", req.Field, "session", st.Session)
		writeJSON(w, http.StatusOK, map[string]any{"session": st.Session, "title": title, "reused": true})
		return
	}

	// Fresh settings discussion through the normal flow.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	title := "settings: " + req.Field
	sess, err := s.spawnSession(ctx, title)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	prime := s.promptWith(discussionPrompt(s.apiBase(), sess.ID), "discussion") +
		"\n\n" + settingsChangeContext(req.Field, value, source, req.Scope, false)
	if err := s.oc.Prompt(ctx, sess.ID, prime); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "prime settings discussion: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.addUnassigned(entry); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	if err := saveSettingsChangeState(s.st.Dir, settingsChangeState{Session: sess.ID}); err != nil {
		slog.Error("settings change state save", "err", err)
	}
	slog.Info("settings discussion created", "field", req.Field, "session", sess.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"session": sess.ID, "title": title, "reused": false})
}

// settingsChangeReusable reports whether sessionID can absorb another
// setting: the session must still exist, and if it is bound to a change
// that change must not be Done or Cancelled.
func (s *Server) settingsChangeReusable(sessionID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.oc.GetSession(ctx, sessionID); err != nil {
		return false
	}
	if changeID, ok := s.sessions.changeOf(sessionID); ok {
		c, err := s.st.Change(changeID)
		if err != nil || c.State == nil {
			return false
		}
		if c.Overall() == model.OverallDone || c.Overall() == model.OverallCancelled {
			return false
		}
	}
	return true
}
