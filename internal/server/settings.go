package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"lessmess/internal/model"
	"lessmess/internal/opencode"
	"lessmess/internal/store"
)

// Settings live in two layered files: lessmess.json at the repository root
// (committed project policy, shared via git) and .lessmess/settings.json
// (gitignored personal override). Effective settings merge per leaf field:
// personal wins over project wins over the built-in default. Both files
// share the Settings schema; every field is optional.
//
// Reads are stateless: every access re-reads the two small files, so edits
// made outside the server (hand edits, git pull) are picked up without a
// watcher or restart. Writes are atomic and confined to the layer being
// saved — the other layer is never touched.

const (
	settingsProjectFile  = "lessmess.json"
	settingsPersonalFile = "settings.json"

	// SettingsScopeProject is the committed root file; SettingsScopePersonal
	// is the gitignored tooling-state file.
	SettingsScopeProject  = "project"
	SettingsScopePersonal = "personal"
)

// errSettingsBadScope is returned for a scope that is neither project nor
// personal; handlers map it to 400.
var errSettingsBadScope = errors.New("settings scope must be project or personal")

// Settings is the schema of both settings files. Booleans are tri-state
// pointers (nil = unset, inherit from the lower layer); empty strings mean
// unset.
type Settings struct {
	Session SessionSettings `json:"session"`
	Prompts PromptSettings  `json:"prompts"`
	Git     GitSettings     `json:"git"`
	UI      UISettings      `json:"ui"`
	Docs    DocsSettings    `json:"docs"`
}

// SessionSettings configure defaults for newly spawned opencode sessions.
// Model is a single "providerID/id" string (model IDs may contain slashes).
type SessionSettings struct {
	Agent            string `json:"agent,omitempty"`
	Model            string `json:"model,omitempty"`
	AutoOpenTerminal *bool  `json:"autoOpenTerminal,omitempty"`
}

// PromptSettings holds free-text addenda appended to the built-in prompts.
// Base prompts are never modified.
type PromptSettings struct {
	Discussion string `json:"discussion,omitempty"`
	Change     string `json:"change,omitempty"`
	Commit     string `json:"commit,omitempty"`
	RepoCommit string `json:"repoCommit,omitempty"`
	Gardener   string `json:"gardener,omitempty"`
	Explorer   string `json:"explorer,omitempty"`
}

// GitSettings configure git-related defaults. DefaultBranch is recorded in
// the root ledger Branch column for newly created changes; it is
// informational only (no branch is created).
type GitSettings struct {
	DefaultBranch string `json:"defaultBranch,omitempty"`
}

// UISettings configure page behavior.
type UISettings struct {
	ShowArchived *bool `json:"showArchived,omitempty"`
}

// DocsSettings configure the docs subsystem.
type DocsSettings struct {
	AutoGardenerOnClose *bool `json:"autoGardenerOnClose,omitempty"`
}

// EffectiveSettings is the concrete view with all defaults materialized.
type EffectiveSettings struct {
	Session EffectiveSessionSettings `json:"session"`
	Prompts PromptSettings           `json:"prompts"`
	Git     GitSettings              `json:"git"`
	UI      EffectiveUISettings      `json:"ui"`
	Docs    EffectiveDocsSettings    `json:"docs"`
}

// EffectiveSessionSettings resolves SessionSettings to concrete values.
type EffectiveSessionSettings struct {
	Agent            string `json:"agent"`
	Model            string `json:"model"`
	AutoOpenTerminal bool   `json:"autoOpenTerminal"`
}

// EffectiveUISettings resolves UISettings to concrete values.
type EffectiveUISettings struct {
	ShowArchived bool `json:"showArchived"`
}

// EffectiveDocsSettings resolves DocsSettings to concrete values.
type EffectiveDocsSettings struct {
	AutoGardenerOnClose bool `json:"autoGardenerOnClose"`
}

// PromptAdd returns the addendum for one named prompt: discussion, change,
// commit, repoCommit, gardener, or explorer. Unknown names yield "".
func (e EffectiveSettings) PromptAdd(name string) string {
	switch name {
	case "discussion":
		return e.Prompts.Discussion
	case "change":
		return e.Prompts.Change
	case "commit":
		return e.Prompts.Commit
	case "repoCommit":
		return e.Prompts.RepoCommit
	case "gardener":
		return e.Prompts.Gardener
	case "explorer":
		return e.Prompts.Explorer
	}
	return ""
}

// settingsState is one load of both layers plus any per-layer parse
// failure, surfaced but never fatal (fail open to defaults).
type settingsState struct {
	Project  Settings
	Personal Settings
	LoadErr  string
}

// settingsProjectPath / settingsPersonalPath locate the two layers.
func settingsProjectPath(repoDir string) string {
	return filepath.Join(repoDir, settingsProjectFile)
}

func settingsPersonalPath(repoDir string) string {
	return filepath.Join(repoDir, store.StateDirName, settingsPersonalFile)
}

// loadSettingsState reads both layers. Missing files are fine; malformed
// files fail open to an empty layer and are recorded in LoadErr.
func loadSettingsState(repoDir string) settingsState {
	var st settingsState
	var errs []string
	for _, layer := range []struct {
		path string
		out  *Settings
	}{
		{settingsProjectPath(repoDir), &st.Project},
		{settingsPersonalPath(repoDir), &st.Personal},
	} {
		s, err := readSettingsLayer(layer.path)
		if err != nil {
			errs = append(errs, err.Error())
		}
		*layer.out = s
	}
	st.LoadErr = strings.Join(errs, "; ")
	return st
}

// readSettingsLayer parses one settings file: missing → zero value and no
// error; unreadable/malformed → zero value and an error.
func readSettingsLayer(path string) (Settings, error) {
	var s Settings
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("%s: %v", filepath.Base(path), err)
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, fmt.Errorf("%s is malformed (%v); using defaults for that layer", filepath.Base(path), err)
	}
	return s, nil
}

// mergeSettings computes the effective view and the per-field source map
// (dotted path → default|project|personal).
func mergeSettings(project, personal Settings) (EffectiveSettings, map[string]string) {
	var eff EffectiveSettings
	sources := map[string]string{}

	pickStr := func(field, proj, pers string) string {
		if pers != "" {
			sources[field] = SettingsScopePersonal
			return pers
		}
		if proj != "" {
			sources[field] = SettingsScopeProject
			return proj
		}
		sources[field] = "default"
		return ""
	}
	pickBool := func(field string, def bool, proj, pers *bool) bool {
		if pers != nil {
			sources[field] = SettingsScopePersonal
			return *pers
		}
		if proj != nil {
			sources[field] = SettingsScopeProject
			return *proj
		}
		sources[field] = "default"
		return def
	}

	eff.Session.Agent = pickStr("session.agent", project.Session.Agent, personal.Session.Agent)
	eff.Session.Model = pickStr("session.model", project.Session.Model, personal.Session.Model)
	eff.Session.AutoOpenTerminal = pickBool("session.autoOpenTerminal", true, project.Session.AutoOpenTerminal, personal.Session.AutoOpenTerminal)

	eff.Prompts.Discussion = pickStr("prompts.discussion", project.Prompts.Discussion, personal.Prompts.Discussion)
	eff.Prompts.Change = pickStr("prompts.change", project.Prompts.Change, personal.Prompts.Change)
	eff.Prompts.Commit = pickStr("prompts.commit", project.Prompts.Commit, personal.Prompts.Commit)
	eff.Prompts.RepoCommit = pickStr("prompts.repoCommit", project.Prompts.RepoCommit, personal.Prompts.RepoCommit)
	eff.Prompts.Gardener = pickStr("prompts.gardener", project.Prompts.Gardener, personal.Prompts.Gardener)
	eff.Prompts.Explorer = pickStr("prompts.explorer", project.Prompts.Explorer, personal.Prompts.Explorer)

	eff.Git.DefaultBranch = pickStr("git.defaultBranch", project.Git.DefaultBranch, personal.Git.DefaultBranch)

	eff.UI.ShowArchived = pickBool("ui.showArchived", true, project.UI.ShowArchived, personal.UI.ShowArchived)
	eff.Docs.AutoGardenerOnClose = pickBool("docs.autoGardenerOnClose", true, project.Docs.AutoGardenerOnClose, personal.Docs.AutoGardenerOnClose)

	return eff, sources
}

// loadEffectiveSettings is the read path for behavior wiring: effective
// view, sources map, and a non-fatal load error string.
func loadEffectiveSettings(repoDir string) (EffectiveSettings, map[string]string, string) {
	st := loadSettingsState(repoDir)
	eff, sources := mergeSettings(st.Project, st.Personal)
	return eff, sources, st.LoadErr
}

// applySettingsPatch validates a settings API body and writes one layer.
// The body is a JSON object whose top-level keys are section names
// ("session", "prompts", "git", "ui", "docs"); each present section
// replaces that whole section in the layer (empty fields are dropped by
// omitempty, so clearing a field restores inheritance). Sections absent
// from the body are left untouched. Unknown sections or fields are
// rejected. The write is atomic.
func applySettingsPatch(repoDir, scope string, body []byte) error {
	var path string
	switch scope {
	case SettingsScopeProject:
		path = settingsProjectPath(repoDir)
	case SettingsScopePersonal:
		path = settingsPersonalPath(repoDir)
	default:
		return errSettingsBadScope
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("%w: settings body must be a JSON object: %v", store.ErrInvalid, err)
	}

	layer, err := readSettingsLayer(path)
	if err != nil {
		// A malformed layer must not block saving: start the layer fresh so
		// the user can fix the file via the UI. The in-flight save only
		// replaces submitted sections, but with an unreadable base the other
		// sections are unrecoverable anyway.
		layer = Settings{}
	}

	for section, payload := range raw {
		if err := patchSection(&layer, section, payload); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(layer, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return model.WriteFileAtomic(path, append(data, '\n'), 0o644)
}

// patchSection replaces one named section of layer with payload, rejecting
// unknown sections and unknown fields within a section.
func patchSection(layer *Settings, section string, payload json.RawMessage) error {
	strict := func(v any) error {
		dec := json.NewDecoder(bytes.NewReader(payload))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			return fmt.Errorf("%w: settings section %q: %v", store.ErrInvalid, section, err)
		}
		return nil
	}
	switch section {
	case "session":
		return strict(&layer.Session)
	case "prompts":
		return strict(&layer.Prompts)
	case "git":
		return strict(&layer.Git)
	case "ui":
		return strict(&layer.UI)
	case "docs":
		return strict(&layer.Docs)
	default:
		return fmt.Errorf("%w: unknown settings section %q", store.ErrInvalid, section)
	}
}

// splitModelRef splits "providerID/id" on the FIRST '/': model IDs may
// themselves contain slashes (e.g. "accounts/fireworks/models/x" under
// provider "fireworks-ai").
func splitModelRef(s string) (providerID, id string) {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

// --- server integration ---

// effectiveSettings loads the current effective settings (stateless read;
// fail open to defaults with a logged warning on a malformed layer).
func (s *Server) effectiveSettings() EffectiveSettings {
	eff, _, loadErr := loadEffectiveSettings(s.st.Dir)
	if loadErr != "" {
		slog.Warn("settings load failed; using defaults", "err", loadErr)
	}
	return eff
}

// promptWith appends the effective addendum for prompt key to base. The
// base prompt is never modified; an empty addendum returns base unchanged.
func (s *Server) promptWith(base, key string) string {
	return appendAddendum(base, promptAddendumFor(s.st.Dir, key))
}

// promptAddendumFor loads the effective addendum for one prompt key in
// repoDir — the stateless companion of Server.promptWith for callers
// without a *Server (the gardener runner).
func promptAddendumFor(repoDir, key string) string {
	eff, _, loadErr := loadEffectiveSettings(repoDir)
	if loadErr != "" {
		slog.Warn("settings load failed; using defaults", "err", loadErr)
	}
	return eff.PromptAdd(key)
}

// appendAddendum appends a settings addendum to a base prompt.
func appendAddendum(base, add string) string {
	if add == "" {
		return base
	}
	return base + "\n\n" + add
}

// spawnSession creates an opencode session titled title, applying the
// effective agent/model defaults at creation.
func (s *Server) spawnSession(ctx context.Context, title string) (*opencode.Session, error) {
	return spawnSession(ctx, s.oc, s.st.Dir, title)
}

// spawnSession creates an opencode session applying the effective
// agent/model defaults. When the service rejects them (400 — typically a
// stale or unknown name), it logs a warning and retries with the service
// defaults so a bad setting never blocks session creation.
func spawnSession(ctx context.Context, oc *opencode.Client, repoDir, title string) (*opencode.Session, error) {
	eff, _, loadErr := loadEffectiveSettings(repoDir)
	if loadErr != "" {
		slog.Warn("settings load failed; using defaults", "err", loadErr)
	}
	var mref *opencode.ModelRef
	if eff.Session.Model != "" {
		prov, id := splitModelRef(eff.Session.Model)
		mref = &opencode.ModelRef{ID: id, ProviderID: prov}
	}
	sess, err := oc.CreateSessionWith(ctx, title, repoDir, eff.Session.Agent, mref)
	if err == nil {
		return sess, nil
	}
	var apiErr *opencode.APIError
	if (eff.Session.Agent != "" || mref != nil) && errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest {
		slog.Warn("session create with configured agent/model rejected; retrying with service defaults",
			"agent", eff.Session.Agent, "model", eff.Session.Model, "err", err)
		return oc.CreateSession(ctx, title, repoDir)
	}
	return nil, err
}
