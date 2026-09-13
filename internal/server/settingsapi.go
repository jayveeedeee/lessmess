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

// putSettings handles PUT /api/settings?scope=project|personal: replace
// the submitted sections in one layer, then return the updated view.
// Submitted agent/model values are validated against the live service
// when it is reachable — the service accepts unknown names at creation
// and then never runs the session, so a typo must be caught here. When
// the service cannot be queried, values are saved unvalidated.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	if verr := s.validateSessionSettings(r.Context(), body); verr != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": verr.Error()})
		return
	}
	if err := applySettingsPatch(s.st.Dir, r.URL.Query().Get("scope"), body); err != nil {
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
	writeJSON(w, http.StatusOK, settingsAPIView(s.st.Dir))
}

// validateSessionSettings checks a PUT body's session section against the
// live service: a non-empty agent must be a primary, non-hidden agent and
// a non-empty model must exist (both scoped to the served repository, so
// project-defined agents/providers count). A nil error means "save". Any
// query failure skips validation — offline saves stay possible.
func (s *Server) validateSessionSettings(ctx context.Context, body []byte) error {
	if s.oc == nil {
		return nil
	}
	var submitted struct {
		Session *SessionSettings `json:"session"`
	}
	if err := json.Unmarshal(body, &submitted); err != nil || submitted.Session == nil {
		return nil // malformed bodies are rejected by applySettingsPatch
	}
	agent, model := strings.TrimSpace(submitted.Session.Agent), strings.TrimSpace(submitted.Session.Model)
	if agent == "" && model == "" {
		return nil
	}
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if agent != "" {
		agents, err := s.oc.ListAgentsFor(qctx, s.st.Dir)
		if err == nil {
			valid := false
			for _, a := range agents {
				if a.ID == agent && a.Mode == "primary" && !a.Hidden {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("unknown agent %q (not a primary agent for this repository)", agent)
			}
		}
	}
	if model != "" {
		models, err := s.oc.ListModelsFor(qctx, s.st.Dir)
		if err == nil {
			valid := false
			for _, m := range models {
				if m.ProviderID+"/"+m.ID == model {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("unknown model %q (not available for this repository)", model)
			}
		}
	}
	return nil
}

// settingsOptionsResponse is the payload of GET /api/settings/options.
// Available is false when the opencode service is unreachable — the page
// then shows a hint and keeps values editable as text.
type settingsOptionsResponse struct {
	Available    bool                `json:"available"`
	Agents       []settingsAgentOpt  `json:"agents"`
	Models       []settingsModelOpt  `json:"models"`
	DefaultModel string              `json:"defaultModel,omitempty"`
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

// settingsOptions handles GET /api/settings/options: live agents and
// models from the opencode service, degrading to available:false.
func (s *Server) settingsOptions(w http.ResponseWriter, r *http.Request) {
	resp := settingsOptionsResponse{Agents: []settingsAgentOpt{}, Models: []settingsModelOpt{}}
	if s.oc == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	agents, err := s.oc.ListAgentsFor(ctx, s.st.Dir)
	if err != nil {
		slog.Warn("settings options: list agents", "err", err)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	models, err := s.oc.ListModelsFor(ctx, s.st.Dir)
	if err != nil {
		slog.Warn("settings options: list models", "err", err)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	for _, a := range agents {
		if a.Mode != "primary" || a.Hidden {
			continue
		}
		resp.Agents = append(resp.Agents, settingsAgentOpt{ID: a.ID, Name: a.Name, Description: a.Description})
	}
	for _, m := range models {
		resp.Models = append(resp.Models, settingsModelOpt{
			ID: m.ID, ProviderID: m.ProviderID, Name: m.Name,
			Value: m.ProviderID + "/" + m.ID,
		})
	}
	if def, err := s.oc.DefaultModel(ctx); err == nil && def != nil && def.ID != "" {
		resp.DefaultModel = def.ProviderID + "/" + def.ID
	}
	resp.Available = true
	writeJSON(w, http.StatusOK, resp)
}
