// Hub mode: one process serving every project registered in the global
// registry, each mounted at /p/<slug>/. The hub owns the root namespace —
// the projects landing page, the registry API, shared static assets, and
// default brand assets — and dispatches everything under /p/ to the
// project's own Server (or setup wizard), exactly as that server would be
// served standalone. Nothing about a project's routes changes; StripPrefix
// removes its mount before the project handler sees the request.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"lessmess/internal/registry"
	"lessmess/internal/store"
)

// BootFunc builds one project's full handler: migrate legacy state, open
// and watch the store, wire the server. basePath is the URL prefix the
// handler is mounted under ("" in single-project mode); publicBase is the
// externally visible base — host:port plus any prefix — baked into
// agent-facing prompts. The returned close func releases the store and its
// watchers; the hub owns it.
type BootFunc func(dir, basePath, publicBase string) (http.Handler, func(), error)

// Slot statuses surfaced by the landing page and the registry API.
const (
	StatusReady       = "ready"
	StatusSetup       = "setup"
	StatusUnavailable = "unavailable"
)

// projectSlot is one mounted project's live state.
type projectSlot struct {
	slug    string
	path    string
	handler http.Handler // nil while unavailable
	close   func()       // nil until a full server booted (directly or via setup hot-swap)
	status  string
	detail  string // boot failure detail for unavailable slots
}

// Hub dispatches /p/<slug>/ to project handlers and serves the global
// surface at the root.
type Hub struct {
	reg  *registry.Store
	boot BootFunc
	host string
	port int
	rend *renderer
	mux  *http.ServeMux

	mu      sync.Mutex
	mountMu sync.Mutex // serializes mounts so self-heal can't double-boot
	slots   map[string]*projectSlot
}

// NewHub builds the hub and boots every registered project immediately.
// Boot failures never abort the hub: a missing or unreadable project stays
// registered and shows as unavailable.
func NewHub(reg *registry.Store, boot BootFunc, host string, port int) *Hub {
	h := &Hub{reg: reg, boot: boot, host: host, port: port, slots: map[string]*projectSlot{}}
	h.rend = newRenderer()
	// The landing page is not a project: default accent (never rolled —
	// there is no repo to persist to) and the product name as the brand.
	h.rend.accent = func() AccentColor { return DefaultAccent() }
	h.rend.projectName = func() string { return "lessmess" }

	m := http.NewServeMux()
	m.HandleFunc("GET /{$}", h.landing)
	m.HandleFunc("GET /api/projects", h.listProjects)
	m.HandleFunc("POST /api/projects", h.addProject)
	m.HandleFunc("DELETE /api/projects/{slug}", h.removeProject)
	if sh, err := staticHandler(); err == nil {
		m.Handle("GET /static/", sh)
	}
	brand := newBrandRenderer(h.rend.accent)
	m.HandleFunc("GET /icon.svg", brand.svg)
	m.HandleFunc("GET /favicon.ico", brand.ico)
	m.HandleFunc("GET /apple-touch-icon.png", brand.touch)
	m.HandleFunc("/p/", h.serveProject)
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		notFoundPage(w, r, "nothing is served at this path on the projects root")
	})
	h.mux = m

	for _, p := range reg.List().Projects {
		h.mount(p)
	}
	return h
}

// Handler returns the root handler.
func (h *Hub) Handler() http.Handler { return h.mux }

// Close releases every mounted project (stores, watchers, SSE fans).
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, slot := range h.slots {
		if slot.close != nil {
			slot.close()
		}
	}
	h.slots = map[string]*projectSlot{}
}

// mount registers a project's slot, serializing against the self-heal
// path (rows → ensureMounted) so the same entry can never be booted twice.
func (h *Hub) mount(p registry.Project) *projectSlot {
	h.mountMu.Lock()
	defer h.mountMu.Unlock()
	return h.mountLocked(p)
}

// ensureMounted reconciles the live slots with the registry file on read
// paths: entries that arrived out-of-band — another hub instance sharing
// the global registry, a hand edit while running — are mounted here
// instead of sitting "unmounted", and slots whose registry entry vanished
// are unmounted. The registry file is the authority; the in-memory table
// converges to it.
func (h *Hub) ensureMounted() {
	h.mountMu.Lock()
	defer h.mountMu.Unlock()
	list := h.reg.List()
	registered := make(map[string]bool, len(list.Projects))
	for _, p := range list.Projects {
		registered[p.Slug] = true
		if slot := h.slot(p.Slug); slot == nil {
			h.mountLocked(p)
		} else if slot.status == StatusUnavailable {
			// A failed boot opened nothing, so re-mounting is free — and
			// once the underlying cause is fixed (a repaired legacy repo,
			// a path that exists again) the project heals on the next
			// landing look instead of at the next restart.
			h.mountLocked(p)
		}
	}
	h.mu.Lock()
	var orphans []string
	for slug := range h.slots {
		if !registered[slug] {
			orphans = append(orphans, slug)
		}
	}
	h.mu.Unlock()
	for _, slug := range orphans {
		h.unmount(slug)
		slog.Info("project unregistered out-of-band; unmounted", "slug", slug)
	}
}

// mountLocked boots one registered project into a slot. Called from
// NewHub for startup boots, from the registry API for hot adds, and from
// ensureMounted for self-heal — mountMu must be held.
func (h *Hub) mountLocked(p registry.Project) *projectSlot {
	slot := &projectSlot{slug: p.Slug, path: p.Path, status: StatusUnavailable}
	if _, err := os.Stat(p.Path); err != nil {
		slot.detail = "directory is not accessible"
		slog.Warn("project unavailable", "slug", p.Slug, "dir", p.Path, "err", err)
		h.addSlot(slot)
		return slot
	}
	basePath := "/p/" + p.Slug
	publicBase := net.JoinHostPort(h.host, fmt.Sprint(h.port)) + basePath
	handler, closeFn, err := h.boot(p.Path, basePath, publicBase)
	switch {
	case err == nil:
		slot.handler, slot.close, slot.status = handler, closeFn, StatusReady
		slog.Info("project mounted", "slug", p.Slug, "dir", p.Path)
	case errors.Is(err, store.ErrNoChanges):
		// Fresh repository: the setup wizard takes the mount and swaps the
		// full server in itself once bootstrap creates changes/.
		slot.handler = NewSetup(p.Path, basePath, func(d string) (http.Handler, error) {
			hh, cl, err := h.boot(d, basePath, publicBase)
			if err != nil {
				return nil, err
			}
			h.mu.Lock()
			if h.slots[p.Slug] != slot {
				// Removed while the wizard was up: release what the boot
				// just opened so nothing outlives the unmount.
				h.mu.Unlock()
				cl()
				return nil, errors.New("project removed during bootstrap")
			}
			slot.close, slot.status = cl, StatusReady
			h.mu.Unlock()
			return hh, nil
		})
		slot.status = StatusSetup
		slog.Info("project mounted in setup mode", "slug", p.Slug, "dir", p.Path)
	default:
		slot.detail = err.Error()
		slog.Error("project boot failed", "slug", p.Slug, "dir", p.Path, "err", err)
	}
	h.addSlot(slot)
	return slot
}

func (h *Hub) addSlot(slot *projectSlot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.slots[slot.slug] = slot
}

func (h *Hub) slot(slug string) *projectSlot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.slots[slug]
}

// serveProject dispatches /p/<slug>/… to the project's handler with the
// mount stripped, so project handlers serve root-shaped requests exactly
// as they would standalone.
func (h *Hub) serveProject(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/p/")
	slug := rest
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		slug = rest[:i]
	}
	if slug == "" {
		notFoundPage(w, r, "no project slug in path")
		return
	}
	// Canonicalize /p/<slug> to /p/<slug>/ so the stripped path is "/".
	if rest == slug {
		target := "/p/" + slug + "/"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}
	slot := h.slot(slug)
	if slot == nil {
		notFoundPage(w, r, "unknown project "+strconv.Quote(slug))
		return
	}
	if slot.handler == nil {
		http.Error(w, "project "+strconv.Quote(slug)+" is unavailable: "+slot.detail, http.StatusServiceUnavailable)
		return
	}
	http.StripPrefix("/p/"+slug, slot.handler).ServeHTTP(w, r)
}

// projectRow is one landing/API row: registry identity plus live status.
type projectRow struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
	BaseURL string `json:"base"`
}

func (h *Hub) rows() []projectRow {
	// Self-heal first: the registry file is the authority, so entries
	// added out-of-band mount here and vanished entries drop their slots.
	h.ensureMounted()
	projects := h.reg.List().Projects
	rows := make([]projectRow, 0, len(projects))
	for _, p := range projects {
		row := projectRow{
			Slug:    p.Slug,
			Name:    effectiveProjectName(p.Path),
			Path:    p.Path,
			BaseURL: "/p/" + p.Slug + "/",
		}
		if slot := h.slot(p.Slug); slot != nil {
			row.Status, row.Detail = slot.status, slot.detail
		} else {
			row.Status, row.Detail = StatusUnavailable, "not mounted"
		}
		rows = append(rows, row)
	}
	return rows
}

// landing serves GET / — the projects list with add and remove.
func (h *Hub) landing(w http.ResponseWriter, r *http.Request) {
	h.rend.render(w, h.rend.projects, "layout", pageData{Title: "projects", Page: "projects", Data: projectsView{Projects: h.rows()}})
}

type projectsView struct {
	Projects []projectRow
}

// listProjects serves GET /api/projects.
func (h *Hub) listProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"projects": h.rows()})
}

// addProject serves POST /api/projects {path}: register the directory in
// the global registry and hot-boot it under its slug without a restart.
// Boot trouble never rolls the registration back — the project shows as
// unavailable, exactly as a startup failure would.
func (h *Hub) addProject(w http.ResponseWriter, r *http.Request) {
	if !isJSON(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON body required"})
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	p, err := h.reg.Add(strings.TrimSpace(req.Path))
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrInvalidPath):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "project path must be absolute"})
		case errors.Is(err, registry.ErrMissing):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "directory does not exist"})
		case errors.Is(err, registry.ErrNotDir):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "path is not a directory"})
		case errors.Is(err, registry.ErrDuplicate):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "this directory is already registered"})
		default:
			writeErr(w, err)
		}
		return
	}
	slot := h.mount(p)
	slog.Info("project added", "slug", p.Slug, "dir", p.Path, "status", slot.status)
	writeJSON(w, http.StatusCreated, map[string]string{"slug": p.Slug, "status": slot.status})
}

// removeProject serves DELETE /api/projects/{slug}: unmount the project
// and delete the registry entry. The repository itself — changes/ and
// .lessmess/ — is never touched. A registry write failure after a
// successful unmount reports 500 and stays retryable (the entry remains,
// unmounted, and a later delete finishes the job).
func (h *Hub) removeProject(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if _, ok := h.reg.Get(slug); !ok {
		notFoundPage(w, r, "unknown project "+strconv.Quote(slug))
		return
	}
	h.unmount(slug)
	if !h.reg.Remove(slug) {
		slog.Error("project unmounted but registry write failed", "slug", slug)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "registry write failed; the project is unmounted but still registered — retry the removal"})
		return
	}
	slog.Info("project removed", "slug", slug)
	w.WriteHeader(http.StatusNoContent)
}

// unmount releases and forgets one project slot. A setup slot whose wizard
// is mid-bootstrap loses the race safely: its boot callback notices the
// slot is gone and closes what it opened.
func (h *Hub) unmount(slug string) {
	h.mu.Lock()
	slot := h.slots[slug]
	delete(h.slots, slug)
	h.mu.Unlock()
	if slot != nil && slot.close != nil {
		slot.close()
	}
}

// notFoundPage writes a 404 that stays readable for both browsers and API
// clients.
func notFoundPage(w http.ResponseWriter, r *http.Request, msg string) {
	if wantsHTML(r) {
		http.Error(w, "404 — "+msg, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": msg})
}
