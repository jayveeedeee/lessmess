package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"lessmess/internal/opencode"
	"lessmess/internal/store"
)

// The settings API serves the settings page and other clients. It exposes
// the effective view plus both raw layers with per-field sources, writes
// one layer at a time, and proxies agent/model options from the opencode
// service (credentials never leave the server).

// settingsResponse is the payload of GET /api/settings (and of a
// successful PUT). Project/Personal are null when that layer's file does
// not exist.
type settingsResponse struct {
	Effective EffectiveSettings `json:"effective"`
	Project   *Settings         `json:"project"`
	Personal  *Settings         `json:"personal"`
	Sources   map[string]string `json:"sources"`
	LoadError string            `json:"loadError,omitempty"`
}

// settingsAPIView builds the current settings payload.
func settingsAPIView(repoDir string) settingsResponse {
	st := loadSettingsState(repoDir)
	eff, sources := mergeSettings(st.Project, st.Personal)
	resp := settingsResponse{Effective: eff, Sources: sources, LoadError: st.LoadErr}
	if fileExists(settingsProjectPath(repoDir)) {
		resp.Project = &st.Project
	}
	if fileExists(settingsPersonalPath(repoDir)) {
		resp.Personal = &st.Personal
	}
	return resp
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// settingsPage handles GET /settings: the settings editor shell. Field
// values, badges, and dropdown options are filled client-side from
// /api/settings and /api/settings/options.
func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	s.rend.render(w, s.rend.settings, "layout", pageData{Title: "settings", Page: "settings"})
}

// getSettings handles GET /api/settings.
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsAPIView(s.st.Dir))
}

// putSettings handles PUT /api/settings?scope=project|personal.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	putSettingsWith(s.oc, s.st.Dir, w, r)
}

// putSettingsWith is the PUT /api/settings handler shared by the normal
// server and the setup-mode shell (which has no store, only a dir):
// replace the submitted sections in one layer, then return the updated
// view. Submitted agent/model values are validated against the live service
// when it is reachable — the service accepts unknown names at creation and
// then never runs the session, so a typo must be caught here. When the
// service cannot be queried, values are saved unvalidated.
func putSettingsWith(oc *opencode.Client, repoDir string, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	if verr := validateSessionSettings(r.Context(), oc, repoDir, body); verr != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": verr.Error()})
		return
	}
	if err := applySettingsPatch(repoDir, r.URL.Query().Get("scope"), body); err != nil {
		switch {
		case errors.Is(err, errSettingsBadScope):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, store.ErrInvalid):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		default:
			slog.Error("settings save", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save settings: " + err.Error()})
		}
		return
	}
	slog.Info("settings saved", "scope", r.URL.Query().Get("scope"))
	writeJSON(w, http.StatusOK, settingsAPIView(repoDir))
}

// listPrimaryAgents returns the primary, non-hidden agents usable for
// sessions in repoDir. The location-scoped list carries project-defined
// agents, but outside the service's home location it omits the built-in
// primaries — which the service nevertheless accepts for sessions in any
// directory — so the default-location list is merged in, deduped by ID
// (scoped entries win).
func listPrimaryAgents(ctx context.Context, oc *opencode.Client, repoDir string) ([]opencode.AgentInfo, error) {
	scoped, err := oc.ListAgentsFor(ctx, repoDir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []opencode.AgentInfo
	for _, a := range scoped {
		if a.Mode != "primary" || a.Hidden || seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		out = append(out, a)
	}
	global, err := oc.ListAgents(ctx)
	if err != nil {
		return out, nil // the scoped list alone is better than failing
	}
	for _, a := range global {
		if a.Mode != "primary" || a.Hidden || seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		out = append(out, a)
	}
	return out, nil
}

// validateSessionSettings checks a PUT body's option-carrying sections:
// a non-empty agent must be a primary, non-hidden agent, non-empty models
// (session.model, docs.gardenerModel) must exist — both scoped to the
// served repository, so project-defined agents/providers count — and a
// non-empty ui.accent must be a palette id. Agent/model validation is
// skipped when the service cannot be queried (offline saves stay
// possible); accent validation is offline and always enforced. A nil
// error means "save".
func validateSessionSettings(ctx context.Context, oc *opencode.Client, repoDir string, body []byte) error {
	var submitted struct {
		Session *SessionSettings `json:"session"`
		Docs    *DocsSettings    `json:"docs"`
		UI      *UISettings      `json:"ui"`
	}
	if err := json.Unmarshal(body, &submitted); err != nil {
		return nil // malformed bodies are rejected by applySettingsPatch
	}
	if submitted.UI != nil {
		if err := validateAccentChoice(strings.TrimSpace(submitted.UI.Accent)); err != nil {
			return err
		}
	}
	if oc == nil {
		return nil
	}
	if submitted.Session == nil && submitted.Docs == nil {
		return nil
	}
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if submitted.Session != nil && strings.TrimSpace(submitted.Session.Agent) != "" {
		agent := strings.TrimSpace(submitted.Session.Agent)
		agents, err := listPrimaryAgents(qctx, oc, repoDir)
		if err == nil {
			valid := false
			for _, a := range agents {
				if a.ID == agent {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("unknown agent %q (not a primary agent for this repository)", agent)
			}
		}
	}
	if submitted.Session != nil {
		if model := strings.TrimSpace(submitted.Session.Model); model != "" {
			if err := validateModelChoice(qctx, oc, repoDir, model); err != nil {
				return err
			}
		}
	}
	if submitted.Docs != nil {
		if model := strings.TrimSpace(submitted.Docs.GardenerModel); model != "" {
			if err := validateModelChoice(qctx, oc, repoDir, model); err != nil {
				return fmt.Errorf("gardener %w", err)
			}
		}
	}
	return nil
}

// validateModelChoice rejects a model name the live service does not offer
// for repoDir. Query failures return nil (offline saves stay possible).
func validateModelChoice(ctx context.Context, oc *opencode.Client, repoDir, model string) error {
	models, err := oc.ListModelsFor(ctx, repoDir)
	if err != nil {
		return nil
	}
	for _, m := range models {
		if m.ProviderID+"/"+m.ID == model {
			return nil
		}
	}
	return fmt.Errorf("unknown model %q (not available for this repository)", model)
}

// settingsOptionsResponse is the payload of GET /api/settings/options.
// Available is false when the opencode service is unreachable — the page
// then shows a hint and keeps values editable as text. Accents is the
// static palette and is always populated.
type settingsOptionsResponse struct {
	Available    bool               `json:"available"`
	Agents       []settingsAgentOpt `json:"agents"`
	Models       []settingsModelOpt `json:"models"`
	Accents      []settingsAccentOpt `json:"accents"`
	DefaultModel string             `json:"defaultModel,omitempty"`
}

// settingsAccentOpt is one palette entry for the accent picker; Hex is
// the dark-theme base value (representative swatch color).
type settingsAccentOpt struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Hex  string `json:"hex"`
}

// settingsAgentOpt is one selectable agent (primary, non-hidden only).
type settingsAgentOpt struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// settingsModelOpt is one selectable model; Value is the pre-joined
// "providerID/id" string stored in settings.
type settingsModelOpt struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Name       string `json:"name"`
	Value      string `json:"value"`
}

// settingsOptions handles GET /api/settings/options.
func (s *Server) settingsOptions(w http.ResponseWriter, r *http.Request) {
	settingsOptionsWith(s.oc, s.st.Dir, w, r)
}

// settingsOptionsWith is the GET /api/settings/options handler shared by
// the normal server and the setup-mode shell: live agents and models from
// the opencode service, degrading to available:false.
func settingsOptionsWith(oc *opencode.Client, repoDir string, w http.ResponseWriter, r *http.Request) {
	resp := settingsOptionsResponse{Agents: []settingsAgentOpt{}, Models: []settingsModelOpt{}, Accents: accentOptions()}
	if oc == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	agents, err := listPrimaryAgents(ctx, oc, repoDir)
	if err != nil {
		slog.Warn("settings options: list agents", "err", err)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	models, err := oc.ListModelsFor(ctx, repoDir)
	if err != nil {
		slog.Warn("settings options: list models", "err", err)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	for _, a := range agents {
		resp.Agents = append(resp.Agents, settingsAgentOpt{ID: a.ID, Name: a.Name, Description: a.Description})
	}
	for _, m := range models {
		resp.Models = append(resp.Models, settingsModelOpt{
			ID: m.ID, ProviderID: m.ProviderID, Name: m.Name,
			Value: m.ProviderID + "/" + m.ID,
		})
	}
	if def, err := oc.DefaultModel(ctx); err == nil && def != nil && def.ID != "" {
		resp.DefaultModel = def.ProviderID + "/" + def.ID
	}
	resp.Available = true
	writeJSON(w, http.StatusOK, resp)
}

// accentOptions renders the static palette for the options endpoint.
func accentOptions() []settingsAccentOpt {
	out := make([]settingsAccentOpt, 0, len(AccentPalette))
	for _, a := range AccentPalette {
		out = append(out, settingsAccentOpt{ID: a.ID, Name: a.Label, Hex: a.Dark})
	}
	return out
}
