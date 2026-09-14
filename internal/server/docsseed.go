package server

// Normal-server docs seed surface: the same resumable seed job the setup
// wizard drives, reachable outside onboarding so an interrupted or
// budget-capped run can be continued with one call. State lives in the
// shared package-level job registry and the .lessmess/docs-seed.json
// cursor — CLI, wizard, and server runs all continue one another.

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"lessmess/internal/docs"
)

// docsSeed handles POST /docs/seed: run (or continue) the docs seed for
// this repository. Without force, the run targets the covered directories
// that do not yet have the doc pair (file-existence based; the cursor is
// ignored for those). With force, every covered directory is redone
// regardless of prior state. 409 when a run is already in flight, 503
// without the opencode integration or docs coverage.
func (s *Server) docsSeed(w http.ResponseWriter, r *http.Request) {
	var req docsSeedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	s.docsSeedWith(w, r, req)
}

// docsSeedWith is the shared start path for POST /docs/seed.
func (s *Server) docsSeedWith(w http.ResponseWriter, r *http.Request, req docsSeedRequest) {
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode integration unavailable"})
		return
	}
	cfg, err := docs.LoadConfig(s.st.Dir)
	if err != nil {
		slog.Error("docs seed: load config", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load coverage config: " + err.Error()})
		return
	}
	if cfg == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "docs coverage disabled (no agentsdocs.json)"})
		return
	}
	opts := docs.SeedOptions{Budget: req.Budget, Force: req.Force}
	if !req.Force {
		// Non-force runs target the dirs missing their doc files; an
		// explicit request overrides the cursor for exactly those.
		missing, err := docs.MissingDocDirs(s.st.Dir)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "find missing docs: " + err.Error()})
			return
		}
		if len(missing) == 0 {
			writeJSON(w, http.StatusOK, map[string]string{"status": "all covered directories have doc files (use force to redo)"})
			return
		}
		opts.Dirs = missing
	}
	root, err := filepath.Abs(s.st.Dir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !startDocsSeedJob(root, cfg, s.oc, opts) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a docs seed is already running"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"ok": "true"})
}

// docsSeedStatus handles GET /docs/seed-status.
func (s *Server) docsSeedStatus(w http.ResponseWriter, r *http.Request) {
	root, err := filepath.Abs(s.st.Dir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, seedJobFor(root).status())
}

// docsExclusions handles GET /docs/exclusions: the top-level coverable
// directories with their exclusion state — the editor's rows. Nested lazy
// expansion remains a wizard capability (GET /api/setup/dirs?dir=…).
func (s *Server) docsExclusions(w http.ResponseWriter, r *http.Request) {
	cfg, err := docs.LoadConfig(s.st.Dir)
	if err != nil {
		slog.Error("docs exclusions: load config", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load coverage config: " + err.Error()})
		return
	}
	entries, err := exclusionDirEntries(s.st.Dir, "", cfg)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such directory"})
		return
	}
	writeJSON(w, http.StatusOK, setupDirsResponse{Dirs: entries, HasConfig: cfg != nil})
}

type exclusionsSaveRequest struct {
	ExcludeDirs []string `json:"excludeDirs"`
}

// docsExclusionsSave handles POST /docs/exclusions: persist the editor's
// selection to agentsdocs.json with the wizard's semantics — picker-
// representable patterns are replaced, hand-authored globs and stale names
// are preserved. 503 without a coverage config (bootstrap owns creation);
// 422 on an invalid pattern.
func (s *Server) docsExclusionsSave(w http.ResponseWriter, r *http.Request) {
	var req exclusionsSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	for _, d := range req.ExcludeDirs {
		if !validExcludePattern(d) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid exclude directory " + strconv.Quote(d)})
			return
		}
	}
	if _, err := os.Stat(filepath.Join(s.st.Dir, docs.ConfigFile)); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "docs coverage disabled (no agentsdocs.json)"})
		return
	}
	saved, err := updateConfigExcludes(s.st.Dir, req.ExcludeDirs)
	if err != nil {
		slog.Error("docs exclusions save", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save exclusions: " + err.Error()})
		return
	}
	if saved {
		slog.Info("docs exclusions saved", "dirs", req.ExcludeDirs)
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": saved})
}
