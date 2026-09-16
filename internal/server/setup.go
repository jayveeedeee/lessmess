package server

// Setup mode: when the served repository has no changes/ tree yet,
// `lessmess serve` starts a first-run wizard surface instead of exiting.
// SetupServer serves only the wizard page, static assets, the settings API
// (the wizard's agent step), and /api/setup/*; once bootstrap creates the
// store, the boot callback (owned by main) builds the full server and the
// root handler swaps over in-process — no restart.
//
// The /setup page and /api/setup/* routes are registered on BOTH the setup
// mux and the normal Server mux via registerSetupRoutes, so the wizard is
// reachable after hot-open and on initialized repos (Settings re-entry)
// with one implementation.

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/model"
	"lessmess/internal/opencode"
)

// setupEnv supplies the shared setup handlers from either the setup shell
// or the normal Server. oc may return nil (integration unavailable). boot
// is nil on the normal server (already booted).
type setupEnv struct {
	dir   string
	rend  *renderer
	oc    func() *opencode.Client
	setOC func(*opencode.Client) // nil on the normal server (SetOpencode owns it)
	boot  func() error
}

// registerSetupRoutes mounts the wizard page and setup API on mux.
func registerSetupRoutes(mux *http.ServeMux, env *setupEnv) {
	mux.HandleFunc("GET /setup", env.wizardPage)
	mux.HandleFunc("GET /api/setup/prereqs", env.prereqs)
	mux.HandleFunc("GET /api/setup/dirs", env.setupDirs)
	mux.HandleFunc("POST /api/setup/bootstrap", env.bootstrap)
	mux.HandleFunc("POST /api/setup/docs-seed", env.docsSeed)
	mux.HandleFunc("GET /api/setup/docs-seed-status", env.docsSeedStatus)
	mux.HandleFunc("POST /api/setup/complete", env.complete)
	mux.HandleFunc("POST /api/setup/dismiss", env.dismiss)
}

// wizardPage serves the setup wizard shell (GET /setup, and GET / in setup
// mode).
func (env *setupEnv) wizardPage(w http.ResponseWriter, r *http.Request) {
	env.rend.render(w, env.rend.setup, "layout", pageData{Title: "setup", Page: "setup"})
}

// Settings API on the setup shell (no store, only a dir) — the wizard's
// agent step reuses the same handlers as the normal server.

func (env *setupEnv) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsAPIView(env.dir))
}

func (env *setupEnv) putSettings(w http.ResponseWriter, r *http.Request) {
	putSettingsWith(env.oc(), env.dir, w, r)
}

func (env *setupEnv) settingsOptions(w http.ResponseWriter, r *http.Request) {
	settingsOptionsWith(env.oc(), env.dir, w, r)
}

// complete marks onboarding finished; the index banner never shows again.
// The body is optional and may carry final step outcomes (e.g. the docs
// step's "skipped").
func (env *setupEnv) complete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Steps map[string]string `json:"steps"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // body optional
	st := loadOnboarding(env.dir)
	for k, v := range req.Steps {
		st.mark(k, v)
	}
	st.CompletedAt = time.Now().Format(time.RFC3339)
	if err := saveOnboarding(env.dir, st); err != nil {
		slog.Error("onboarding complete", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save onboarding state"})
		return
	}
	slog.Info("onboarding completed", "dir", env.dir)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// dismiss suppresses the index banner without completing onboarding.
func (env *setupEnv) dismiss(w http.ResponseWriter, r *http.Request) {
	st := loadOnboarding(env.dir)
	st.Dismissed = true
	if err := saveOnboarding(env.dir, st); err != nil {
		slog.Error("onboarding dismiss", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save onboarding state"})
		return
	}
	slog.Info("onboarding dismissed", "dir", env.dir)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// --- bootstrap ---

// setupDirEntry is one directory offered for docs exclusion. Rel is the
// repo-relative path (slash-separated); Excluded reflects current user
// config patterns; HasChildren marks an expandable row.
type setupDirEntry struct {
	Name            string `json:"name"`
	Rel             string `json:"rel"`
	DefaultExcluded bool   `json:"defaultExcluded"` // matches docs.DefaultExclude: excluded no matter what
	Excluded        bool   `json:"excluded"`
	HasChildren     bool   `json:"hasChildren"`
}

// setupDirsResponse is the payload of GET /api/setup/dirs.
type setupDirsResponse struct {
	Dir       string          `json:"dir"`
	Dirs      []setupDirEntry `json:"dirs"`
	HasConfig bool            `json:"hasConfig"`
}

// setupDirs lists the immediate coverable subdirectories of ?dir= (default
// the repo root) for the exclusion picker: non-hidden, never changes/ or
// .lessmess/. Children load lazily as the user expands rows.
func (env *setupEnv) setupDirs(w http.ResponseWriter, r *http.Request) {
	rel := strings.Trim(r.URL.Query().Get("dir"), "/")
	if !validSetupRel(rel) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "bad dir " + strconv.Quote(rel)})
		return
	}
	base := env.dir
	if rel != "" {
		base = filepath.Join(env.dir, filepath.FromSlash(rel))
	}
	cfg, cfgErr := docs.LoadConfig(env.dir)
	if cfgErr != nil {
		slog.Warn("setup dirs: config unreadable", "err", cfgErr)
	}
	entries, err := exclusionDirEntries(base, rel, cfg)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such directory"})
		return
	}
	writeJSON(w, http.StatusOK, setupDirsResponse{Dir: rel, Dirs: entries, HasConfig: cfg != nil})
}

// exclusionDirEntries lists base's immediate coverable subdirectories with
// their exclusion state (the picker's rows): non-hidden, never changes/.
// The wizard picker and the normal server's exclusions editor share it.
func exclusionDirEntries(base, rel string, cfg *docs.Config) ([]setupDirEntry, error) {
	ents, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var out []setupDirEntry
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		childRel := e.Name()
		if rel != "" {
			childRel = rel + "/" + e.Name()
		}
		if childRel == "changes" {
			continue
		}
		entry := setupDirEntry{Name: e.Name(), Rel: childRel}
		for _, x := range docs.DefaultExclude {
			if ok, _ := path.Match(x, e.Name()); ok {
				entry.DefaultExcluded = true
				break
			}
		}
		if cfg != nil && cfg.UserExcluded(childRel) {
			entry.Excluded = true
		}
		if !entry.DefaultExcluded {
			entry.HasChildren = hasSubdirs(filepath.Join(base, e.Name()))
		}
		out = append(out, entry)
	}
	return out, nil
}

// validSetupRel guards the ?dir= parameter: repo-relative, no escapes.
func validSetupRel(rel string) bool {
	if rel == "" {
		return true
	}
	for _, s := range strings.Split(rel, "/") {
		if s == "" || s == "." || s == ".." || strings.HasPrefix(s, ".") || strings.Contains(s, `\`) {
			return false
		}
	}
	return strings.Split(rel, "/")[0] != "changes"
}

// hasSubdirs reports whether abs contains at least one non-hidden
// subdirectory (one level — the picker expands lazily).
func hasSubdirs(abs string) bool {
	ents, err := os.ReadDir(abs)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			return true
		}
	}
	return false
}

// validExcludePattern validates one picker-submitted exclude: slash-
// separated plain directory names; no escapes, hidden segments, globs, or
// the changes/ tree.
func validExcludePattern(d string) bool {
	if strings.ContainsAny(d, "?*[{|\\") {
		return false
	}
	segs := strings.Split(d, "/")
	for _, s := range segs {
		if s == "" || s == "." || s == ".." || strings.HasPrefix(s, ".") {
			return false
		}
	}
	return segs[0] != "changes"
}

type bootstrapRequest struct {
	// DocsCoverage is tri-state: absent means enabled (the wizard default).
	DocsCoverage *bool `json:"docsCoverage"`
	// ExcludeDirs holds user-chosen directory patterns written into the
	// coverage config: base names (top-level picks, match same-named dirs
	// at any depth) or slash paths (nested picks, match the exact subtree).
	ExcludeDirs []string `json:"excludeDirs"`
}

type bootstrapResponse struct {
	Actions  []docs.InitAction `json:"actions"`
	Reloaded bool              `json:"reloaded"`
}

// updateConfigExcludes rewrites the user excludes of an existing coverage
// config with the picker's selection. Patterns the picker cannot represent
// — glob metachars, or names/paths with no matching directory on disk
// (stale or hand-authored, e.g. "web/static" when that path is absent) —
// are preserved; everything else is replaced by the submitted set, so
// previous picker selections round-trip (and deselect cleanly).
func updateConfigExcludes(dir string, excludes []string) (bool, error) {
	p := filepath.Join(dir, docs.ConfigFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return false, nil
	}
	var raw struct {
		Include []string `json:"include"`
		Exclude []string `json:"exclude"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	kept := []string{}
	for _, x := range raw.Exclude {
		if pickerOpaque(x, dir) {
			kept = append(kept, x)
		}
	}
	raw.Exclude = append(kept, normalizeExcludes(excludes)...)
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return false, err
	}
	if bytes.Equal(bytes.TrimSpace(data), bytes.TrimSpace(out)) {
		return false, nil
	}
	return true, model.WriteFileAtomic(p, append(out, '\n'), 0o644)
}

// pickerOpaque reports whether an exclude pattern cannot be produced or
// edited by the picker: it contains glob metachars, or matches no existing
// directory (base names look for a same-named top-level dir, slash paths
// for the exact one).
func pickerOpaque(pattern, root string) bool {
	if strings.ContainsAny(pattern, "?*[{") {
		return true
	}
	st, err := os.Stat(filepath.Join(root, filepath.FromSlash(strings.Trim(pattern, "/"))))
	return err != nil || !st.IsDir()
}

// normalizeExcludes drops submitted patterns whose ancestor is also
// submitted (redundant: the parent already prunes the subtree).
func normalizeExcludes(excludes []string) []string {
	set := map[string]bool{}
	for _, e := range excludes {
		set[e] = true
	}
	var out []string
	for _, e := range excludes {
		redundant := false
		for p := e; ; {
			i := strings.LastIndex(p, "/")
			if i < 0 {
				break
			}
			p = p[:i]
			if set[p] {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, e)
		}
	}
	return out
}

// bootstrap runs docs.Init from the wizard: merge-safe repository
// bootstrap with a separate docs-coverage choice, then (setup mode only)
// the hot-open boot that swaps in the full server. On the normal server it
// simply re-runs idempotently with reloaded:false.
func (env *setupEnv) bootstrap(w http.ResponseWriter, r *http.Request) {
	var req bootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	coverage := req.DocsCoverage == nil || *req.DocsCoverage
	var excludes []string
	if coverage {
		for _, d := range req.ExcludeDirs {
			d = strings.Trim(strings.TrimSpace(d), "/")
			if d == "" {
				continue
			}
			// The picker sends concrete directory names ("docs") or paths
			// ("src/generated"); reject anything else since this writes a
			// committed config file.
			if !validExcludePattern(d) {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid exclude directory " + strconv.Quote(d)})
				return
			}
			excludes = append(excludes, d)
		}
	}
	actions, err := docs.InitWithOptions(env.dir, docs.InitOptions{Config: coverage, Exclude: excludes})
	if err != nil {
		slog.Error("bootstrap", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "actions": actions})
		return
	}
	for _, a := range actions {
		slog.Info("bootstrap", "action", a.Action, "path", a.Path)
	}

	// An explicit exclusion submission also updates an EXISTING config
	// (init only writes the file at creation); this is how an already
	// bootstrapped repo changes its excludes from the wizard.
	if coverage {
		if merged, err := updateConfigExcludes(env.dir, excludes); err != nil {
			slog.Warn("bootstrap: update config excludes", "err", err)
		} else if merged {
			for i, a := range actions {
				if a.Path == docs.ConfigFile && a.Action == "skipped" {
					actions[i].Action = "merged"
				}
			}
			slog.Info("bootstrap", "action", "merged", "path", docs.ConfigFile)
		}
	}

	st := loadOnboarding(env.dir)
	st.mark("bootstrap", "done")
	if coverage {
		st.mark("docs-coverage", "enabled")
	} else {
		st.mark("docs-coverage", "disabled")
	}
	if err := saveOnboarding(env.dir, st); err != nil {
		slog.Warn("onboarding state save", "err", err)
	}

	// Hot-open only makes sense in setup mode and only once changes/ exists.
	reloaded := false
	if env.boot != nil {
		if st, err := os.Stat(filepath.Join(env.dir, "changes")); err == nil && st.IsDir() {
			if err := env.boot(); err != nil {
				slog.Error("hot-open after bootstrap", "err", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "bootstrap succeeded but reload failed: " + err.Error()})
				return
			}
			reloaded = true
		}
	}
	writeJSON(w, http.StatusOK, bootstrapResponse{Actions: actions, Reloaded: reloaded})
}

// SetupServer is the setup-mode root handler: it serves the setup mux until
// a successful boot swaps in the full server handler, then delegates.
type SetupServer struct {
	env *setupEnv
	mux http.Handler

	mu      sync.Mutex
	oc      *opencode.Client // discovered by the prereq re-check (ONB-02)
	bootFn  func(dir string) (http.Handler, error)
	swapped atomic.Value // http.Handler after a successful boot
}

// NewSetup builds the setup-mode handler for dir. boot builds the full
// handler once the repository is initialized (see main.runServe).
func NewSetup(dir string, boot func(string) (http.Handler, error)) *SetupServer {
	s := &SetupServer{bootFn: boot}
	s.env = &setupEnv{
		dir:   dir,
		rend:  newRenderer(),
		oc:    s.ocClient,
		setOC: s.setOCClient,
		boot:  s.tryBoot,
	}

	m := http.NewServeMux()
	registerSetupRoutes(m, s.env)
	m.HandleFunc("GET /{$}", s.env.wizardPage)
	// The wizard's agent step reuses the settings API; the setup shell has
	// no store, so these thin env handlers serve it with just the dir.
	m.HandleFunc("GET /api/settings", s.env.getSettings)
	m.HandleFunc("PUT /api/settings", s.env.putSettings)
	m.HandleFunc("GET /api/settings/options", s.env.settingsOptions)
	// Brand assets resolve read-only here: setup mode never rolls an
	// accent (no store yet), so the wizard shows the default orange. The
	// project name reads the same layered settings the wizard edits.
	s.env.rend.accent = func() AccentColor { return effectiveAccentColor(dir) }
	s.env.rend.projectName = func() string { return effectiveProjectName(dir) }
	brand := newBrandRenderer(s.env.rend.accent)
	m.HandleFunc("GET /icon.svg", brand.svg)
	m.HandleFunc("GET /favicon.ico", brand.ico)
	m.HandleFunc("GET /apple-touch-icon.png", brand.touch)
	if sh, err := staticHandler(); err == nil {
		m.Handle("GET /static/", sh)
	}
	m.HandleFunc("/", s.guard)
	s.mux = m
	return s
}

// ocClient is the setupEnv opencode accessor for the setup shell.
func (s *SetupServer) ocClient() *opencode.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.oc
}

// setOCClient stores a client discovered by the prereq re-check.
func (s *SetupServer) setOCClient(c *opencode.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.oc = c
}

// tryBoot runs the boot callback exactly once; later calls are no-ops.
func (s *SetupServer) tryBoot() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.swapped.Load() != nil || s.bootFn == nil {
		return nil
	}
	h, err := s.bootFn(s.env.dir)
	if err != nil {
		return err
	}
	s.swapped.Store(h)
	slog.Info("setup boot complete; serving full UI", "dir", s.env.dir)
	return nil
}

// ServeHTTP delegates to the full server after boot, else the setup mux.
func (s *SetupServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h, ok := s.swapped.Load().(http.Handler); ok {
		h.ServeHTTP(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// guard refuses everything outside the setup surface: HTML page GETs
// redirect to the wizard, everything else gets 503 JSON.
func (s *SetupServer) guard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && wantsHTML(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "setup not complete"})
}
