package server

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"lessmess/internal/docs"
)

// docsRuntimeState is the single view of the experimental docs feature used
// by runtime guards and the Settings status endpoint. Enabled and
// config state are live reads; Active reflects the startup-only queue/watcher
// wiring. A mismatch therefore requires a process restart.
type docsRuntimeState struct {
	Enabled         bool   `json:"enabled"`
	ConfigExists    bool   `json:"configExists"`
	ConfigReadable  bool   `json:"configReadable"`
	Active          bool   `json:"active"`
	RestartRequired bool   `json:"restartRequired"`
	ConfigError     string `json:"configError,omitempty"`
}

// docsStatus returns the live setting/config state alongside the startup-only
// runtime state. The Settings page uses it to distinguish initialization from
// repair and restart actions.
func (s *Server) docsStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.docsRuntimeState())
}

// docsInitialize creates the committed coverage config from Settings. It is
// intentionally idempotent and never overwrites an existing (even malformed)
// file; malformed configs need an explicit human repair.
func (s *Server) docsInitialize(w http.ResponseWriter, r *http.Request) {
	state := s.docsRuntimeState()
	if !state.Enabled {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "docs are disabled; save Enable docs as On first"})
		return
	}
	if state.ConfigError != "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "agentsdocs.json exists but is unreadable; fix or remove it before initializing"})
		return
	}
	if state.ConfigReadable {
		writeJSON(w, http.StatusOK, map[string]any{"created": false, "status": state})
		return
	}

	action, err := docs.InitConfig(s.st.Dir)
	if err != nil {
		slog.Error("initialize docs coverage", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "initialize docs coverage: " + err.Error()})
		return
	}
	state = s.docsRuntimeState()
	if state.ConfigError != "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "agentsdocs.json exists but is unreadable; fix or remove it before initializing"})
		return
	}
	status := http.StatusOK
	if action.Action == "created" {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"created": action.Action == "created", "status": state})
}

func (s *Server) docsRuntimeState() docsRuntimeState {
	state := docsRuntimeState{
		Enabled: DocsEnabled(s.st.Dir),
		Active:  s.docsQ != nil,
	}
	configPath := filepath.Join(s.st.Dir, docs.ConfigFile)
	if _, err := os.Stat(configPath); err == nil {
		state.ConfigExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		state.ConfigError = err.Error()
	}

	cfg, err := docs.LoadConfig(s.st.Dir)
	if err != nil {
		state.ConfigExists = true
		state.ConfigError = err.Error()
	} else if cfg != nil {
		state.ConfigExists = true
		state.ConfigReadable = true
	}
	desiredActive := state.Enabled && state.ConfigReadable
	state.RestartRequired = desiredActive != state.Active
	return state
}

func (s *Server) docsInactiveMessage() string {
	state := s.docsRuntimeState()
	if state.Active {
		return ""
	}
	if !state.Enabled {
		return "docs are disabled; enable Experimental Docs in Settings → Docs, then restart lessmess"
	}
	if state.ConfigError != "" {
		return "docs coverage config is unreadable; fix agentsdocs.json, then restart lessmess"
	}
	if !state.ConfigExists {
		return "docs coverage is not initialized; initialize it in Settings → Docs, then restart lessmess"
	}
	return "docs are configured but not active; restart lessmess"
}
