package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"lessmess/internal/model"
)

// The served repository's opencode.json can declare its own default agent
// ("default_agent"). opencode applies it to every session created without
// an explicit agent — including sessions made in the opencode TUI/CLI,
// which never see lessmess's session.agent. Surfacing the divergence (and,
// in SPF-03, an explicit align action) keeps the two defaults from
// silently disagreeing.

// opencodeConfigFile is the project-level opencode config lessmess reads
// (and, on explicit align, patches). Only the plain-JSON spelling is
// handled: JSONC comments and trailing commas cannot be round-tripped by
// the standard library, so such files are reported unreadable and never
// rewritten.
const opencodeConfigFile = "opencode.json"

// Opencode default statuses.
const (
	ocDefaultOK         = "ok"         // file parsed and declares default_agent
	ocDefaultAbsent     = "absent"     // no file, or no default_agent key
	ocDefaultUnreadable = "unreadable" // present but not plain JSON
)

// opencodeDefault describes the repository's declared opencode default
// agent for the settings payload.
type opencodeDefault struct {
	// Declared is the default_agent value; empty unless Status is ok.
	Declared string `json:"declared,omitempty"`
	// Path is the config file's repository-relative location.
	Path string `json:"path"`
	// Status is ok, absent, or unreadable.
	Status string `json:"status"`
}

func opencodeConfigPath(repoDir string) string {
	return filepath.Join(repoDir, opencodeConfigFile)
}

// readOpencodeDefault inspects the repository's opencode.json. All failures
// degrade to a status report — the notice is advisory and must never break
// the settings payload.
func readOpencodeDefault(repoDir string) opencodeDefault {
	rel := opencodeConfigFile
	path := opencodeConfigPath(repoDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return opencodeDefault{Path: rel, Status: ocDefaultAbsent}
	}
	var cfg struct {
		DefaultAgent string `json:"default_agent"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Comments or trailing commas (JSONC) land here; Go's JSON cannot
		// distinguish them from corruption, and both are unpatchable.
		slog.Debug("opencode.json not plain JSON", "path", path, "err", err)
		return opencodeDefault{Path: rel, Status: ocDefaultUnreadable}
	}
	if cfg.DefaultAgent == "" {
		return opencodeDefault{Path: rel, Status: ocDefaultAbsent}
	}
	return opencodeDefault{Declared: cfg.DefaultAgent, Path: rel, Status: ocDefaultOK}
}

// patchDefaultAgent replaces the value of the top-level default_agent key,
// preserving every other byte of the file — key order, whitespace, and all
// unknown keys — since opencode.json is committed project config. The input
// must be plain JSON: comments or trailing commas (JSONC) are an error,
// because the file could not be rewritten safely.
func patchDefaultAgent(data []byte, newAgent string) (patched []byte, old string, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, "", fmt.Errorf("not plain JSON: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, "", errors.New("not a JSON object")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, "", fmt.Errorf("not plain JSON: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, "", errors.New("unexpected key token")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, "", fmt.Errorf("not plain JSON: %w", err)
		}
		if key != "default_agent" {
			continue
		}
		end := dec.InputOffset()
		start := end - int64(len(raw))
		var old string
		if start < 0 || end > int64(len(data)) || start > end ||
			!bytes.Equal(data[start:end], raw) ||
			json.Unmarshal(raw, &old) != nil {
			return nil, "", errors.New("cannot locate the default_agent value")
		}
		lit, err := json.Marshal(newAgent)
		if err != nil {
			return nil, "", err
		}
		out := make([]byte, 0, len(data)-len(raw)+len(lit))
		out = append(out, data[:start]...)
		out = append(out, lit...)
		out = append(out, data[end:]...)
		return out, old, nil
	}
	return nil, "", errors.New("opencode.json has no default_agent key")
}

// alignOpencodeDefault handles POST /api/settings/opencode-default-agent:
// point the repository's opencode.json default_agent at the effective
// session.agent. The write is explicit (a user click) and therefore
// strictly validated — the agent must be a known primary agent of the live
// service (503 when it cannot be queried) — and the file is never
// rewritten unless it is plain JSON with a default_agent key.
func (s *Server) alignOpencodeDefault(w http.ResponseWriter, r *http.Request) {
	repoDir := s.st.Dir
	agent := strings.TrimSpace(settingsAPIView(repoDir).Effective.Session.Agent)
	if agent == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "no session.agent configured — set a default agent above first"})
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service not configured"})
		return
	}
	agents, err := listPrimaryAgents(r.Context(), s.oc, repoDir)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unreachable: " + err.Error()})
		return
	}
	known := false
	for _, a := range agents {
		if a.ID == agent {
			known = true
			break
		}
	}
	if !known {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": fmt.Sprintf("agent %q is not a known primary agent of the opencode service", agent)})
		return
	}
	path := opencodeConfigPath(repoDir)
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "cannot read opencode.json: " + err.Error()})
		return
	}
	patched, old, perr := patchDefaultAgent(data, agent)
	if perr != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": perr.Error() + " — set default_agent manually"})
		return
	}
	if err := model.WriteFileAtomic(path, patched, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write opencode.json: " + err.Error()})
		return
	}
	slog.Info("opencode.json default_agent aligned", "from", old, "to", agent)
	writeJSON(w, http.StatusOK, map[string]string{"old": old, "new": agent})
}
