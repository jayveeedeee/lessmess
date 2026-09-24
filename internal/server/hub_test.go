package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"lessmess/internal/registry"
	"lessmess/internal/store"
)

// hubBoot mirrors main's boot closure for tests: open the store and build
// the full handler. The returned close func owns the store — the hub calls
// it from Close, so tests must not register their own cleanup for these.
func hubBoot(t *testing.T) BootFunc {
	t.Helper()
	return func(dir, basePath, publicBase string) (http.Handler, func(), error) {
		st, err := store.Open(dir)
		if err != nil {
			return nil, nil, err
		}
		if err := st.Watch(context.Background()); err != nil {
			st.Close()
			return nil, nil, err
		}
		app := New(st)
		app.Base = basePath
		app.PublicBase = publicBase
		return app.Handler(), func() { app.Close(); st.Close() }, nil
	}
}

// hubFixture registers four projects and builds a hub over them: two full
// fixture repos (a carries one extra change for isolation checks), one
// empty directory (setup mode), one missing directory (unavailable).
func hubFixture(t *testing.T) *Hub {
	t.Helper()
	a, b := fixtureRepo(t), fixtureRepo(t)
	addFixtureChange(t, a, "2026-09-11-1", "Only in A")
	fresh := t.TempDir()
	ghost := filepath.Join(t.TempDir(), "gone")

	cfg := registry.Config{Projects: []registry.Project{
		{Slug: "alpha", Path: a, Added: "2026-09-24"},
		{Slug: "beta", Path: b, Added: "2026-09-24"},
		{Slug: "fresh", Path: fresh, Added: "2026-09-24"},
		{Slug: "ghost", Path: ghost, Added: "2026-09-24"},
	}}
	reg := registry.OpenStore(filepath.Join(t.TempDir(), "config.json"))
	if err := registry.Save(reg.Path(), cfg); err != nil {
		t.Fatal(err)
	}
	return NewHub(reg, hubBoot(t), "127.0.0.1", 9099)
}

func TestHubMountsProjects(t *testing.T) {
	h := hubFixture(t)
	defer h.Close()

	// Landing lists every registered project with its mount URL.
	w := do(t, h.Handler(), "GET", "/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("landing = %d", w.Code)
	}
	for _, want := range []string{"/p/alpha/", "/p/beta/", "alpha", "beta"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("landing missing %q", want)
		}
	}

	// Registry API: statuses cover ready, setup, and unavailable.
	w = do(t, h.Handler(), "GET", "/api/projects", "")
	if w.Code != http.StatusOK {
		t.Fatalf("api/projects = %d %s", w.Code, w.Body)
	}
	var resp struct {
		Projects []projectRow `json:"projects"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	got := map[string]projectRow{}
	for _, p := range resp.Projects {
		got[p.Slug] = p
	}
	if got["alpha"].Status != StatusReady || got["beta"].Status != StatusReady {
		t.Errorf("full repos: alpha=%q beta=%q, want ready", got["alpha"].Status, got["beta"].Status)
	}
	if got["fresh"].Status != StatusSetup {
		t.Errorf("fresh = %q, want setup", got["fresh"].Status)
	}
	if got["ghost"].Status != StatusUnavailable {
		t.Errorf("ghost = %q, want unavailable", got["ghost"].Status)
	}
	if got["alpha"].Name == "" {
		t.Error("alpha row carries no display name")
	}
	if got["alpha"].BaseURL != "/p/alpha/" {
		t.Errorf("alpha base = %q, want /p/alpha/", got["alpha"].BaseURL)
	}
}

func TestHubProjectRequestsStripPrefix(t *testing.T) {
	h := hubFixture(t)
	defer h.Close()

	// A project page answers with root-shaped routing under the mount.
	w := do(t, h.Handler(), "GET", "/p/alpha/", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Fixture change") {
		t.Fatalf("/p/alpha/ = %d, want the project index", w.Code)
	}
	w = do(t, h.Handler(), "GET", "/p/alpha/changes/2026-09-10-0", "")
	if w.Code != http.StatusOK {
		t.Fatalf("/p/alpha/changes/… = %d", w.Code)
	}
	// The mount disappears for JSON routes too.
	w = do(t, h.Handler(), "GET", "/p/alpha/api/git/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("/p/alpha/api/git/status = %d", w.Code)
	}
	// Bare /p/alpha redirects to the trailing slash.
	w = do(t, h.Handler(), "GET", "/p/alpha", "")
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "/p/alpha/" {
		t.Fatalf("/p/alpha = %d %s, want redirect to /p/alpha/", w.Code, w.Header().Get("Location"))
	}
}

func TestHubProjectsAreIsolated(t *testing.T) {
	h := hubFixture(t)
	defer h.Close()

	// The change that exists only in alpha 404s through beta's mount.
	w := do(t, h.Handler(), "GET", "/p/beta/changes/2026-09-11-1", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-project change = %d, want 404", w.Code)
	}
	// And alpha's own board still sees it.
	w = do(t, h.Handler(), "GET", "/p/alpha/changes/2026-09-11-1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("owning project = %d, want 200", w.Code)
	}
}

func TestHubUnknownAndUnavailable(t *testing.T) {
	h := hubFixture(t)
	defer h.Close()

	w := do(t, h.Handler(), "GET", "/p/missing/", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown slug = %d, want 404", w.Code)
	}
	w = do(t, h.Handler(), "GET", "/p/ghost/", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("ghost = %d, want 503", w.Code)
	}
	// The root namespace serves nothing else — API paths stay project-local.
	w = do(t, h.Handler(), "GET", "/api/validate", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("root /api/validate = %d, want 404", w.Code)
	}
}

func TestHubSetupProjectHotSwapsUnderPrefix(t *testing.T) {
	// The fresh project's wizard lives under its mount; after bootstrap it
	// serves the full server without re-mounting.
	h := hubFixture(t)
	defer h.Close()

	w := do(t, h.Handler(), "POST", "/p/fresh/api/setup/bootstrap", `{"docsCoverage":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body)
	}
	var resp bootstrapResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Reloaded {
		t.Fatal("reloaded = false, want hot-open under the prefix")
	}
	// The slot now reports ready and the mount serves the project index.
	if s := h.slot("fresh"); s.status != StatusReady {
		t.Fatalf("fresh slot status = %q, want ready", s.status)
	}
	w = do(t, h.Handler(), "GET", "/p/fresh/", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "changes") {
		t.Fatalf("/p/fresh/ after bootstrap = %d, want the project index", w.Code)
	}
}

func TestHubCloseReleasesProjects(t *testing.T) {
	h := hubFixture(t)
	h.Close()
	// Double close is safe, and a closed hub no longer dispatches: slots
	// are gone, so a project request 404s instead of touching a dead store.
	h.Close()
	w := do(t, h.Handler(), "GET", "/p/alpha/", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("post-close request = %d, want 404 (slots released)", w.Code)
	}
}

func TestHubAddRemoveProjects(t *testing.T) {
	root := t.TempDir()
	reg := registry.OpenStore(filepath.Join(root, "config.json"))
	h := NewHub(reg, hubBoot(t), "127.0.0.1", 9099)
	defer h.Close()

	// Empty registry: the landing shows the empty state.
	if w := htmlGet(t, h.Handler(), "/", false); !strings.Contains(w.Body.String(), "No projects registered yet") {
		t.Fatalf("landing = %s", w.Body)
	}

	// Add a full fixture repo → hot-booted under its slug.
	repo := fixtureRepo(t)
	w := do(t, h.Handler(), "POST", "/api/projects", `{"path":"`+repo+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", w.Code, w.Body)
	}
	var created struct{ Slug, Status string }
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != StatusReady {
		t.Fatalf("status = %q, want ready", created.Status)
	}
	if w = htmlGet(t, h.Handler(), "/p/"+created.Slug+"/", false); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Fixture change") {
		t.Fatalf("/p/%s/ = %d, want the booted project", created.Slug, w.Code)
	}
	// The registry file on disk carries the entry.
	if got := len(registry.Load(reg.Path()).Projects); got != 1 {
		t.Fatalf("registry file has %d entries, want 1", got)
	}

	// Same directory again → conflict.
	if w = do(t, h.Handler(), "POST", "/api/projects", `{"path":"`+repo+`"}`); w.Code != http.StatusConflict {
		t.Fatalf("re-add = %d, want 409", w.Code)
	}

	// Invalid submissions → 422; malformed bodies → 400.
	file := filepath.Join(t.TempDir(), "f.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	for _, tc := range []struct{ name, body string }{
		{"relative", `{"path":"rel/path"}`},
		{"missing", `{"path":"` + filepath.Join(root, "absent") + `"}`},
		{"file", `{"path":"` + file + `"}`},
	} {
		if w = do(t, h.Handler(), "POST", "/api/projects", tc.body); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d, want 422", tc.name, w.Code)
		}
	}
	if w = do(t, h.Handler(), "POST", "/api/projects", `{not json`); w.Code != http.StatusBadRequest {
		t.Errorf("bad json = %d, want 400", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/projects", strings.NewReader("path=x"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Handler().ServeHTTP(w2, r)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("non-json = %d, want 400", w2.Code)
	}

	// An empty directory registers and mounts its setup wizard.
	fresh := t.TempDir()
	w = do(t, h.Handler(), "POST", "/api/projects", `{"path":"`+fresh+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("fresh add = %d %s", w.Code, w.Body)
	}
	var freshResp struct{ Slug, Status string }
	json.Unmarshal(w.Body.Bytes(), &freshResp)
	if freshResp.Status != StatusSetup {
		t.Fatalf("fresh status = %q, want setup", freshResp.Status)
	}

	// Unknown slug → 404.
	if w = do(t, h.Handler(), "DELETE", "/api/projects/absent", ""); w.Code != http.StatusNotFound {
		t.Fatalf("delete unknown = %d, want 404", w.Code)
	}

	// Delete the ready project: unmounted, forgotten, repo untouched.
	idxPath := filepath.Join(repo, ".lessmess", "workflow", "index.json")
	if w = do(t, h.Handler(), "DELETE", "/api/projects/"+created.Slug, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body)
	}
	if w = do(t, h.Handler(), "GET", "/p/"+created.Slug+"/", ""); w.Code != http.StatusNotFound {
		t.Fatalf("post-delete mount = %d, want 404", w.Code)
	}
	if rows := registry.Load(reg.Path()).Projects; len(rows) != 1 {
		t.Fatalf("registry rows = %d, want 1 (the fresh setup project)", len(rows))
	}
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("repository state touched by remove: %v", err)
	}

	// Re-adding the same directory gets the same slug back.
	w = do(t, h.Handler(), "POST", "/api/projects", `{"path":"`+repo+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("re-add after remove = %d", w.Code)
	}
	var reAdded struct{ Slug string }
	json.Unmarshal(w.Body.Bytes(), &reAdded)
	if reAdded.Slug != created.Slug {
		t.Fatalf("re-add slug = %q, want %q", reAdded.Slug, created.Slug)
	}
}

// The project level lives in the breadcrumb trail, not a separate header
// control: the page exposes its mount (data-base) and display name
// (data-project-name) for the client-built trail, and no project dropdown
// markup exists anywhere.
func TestProjectBreadcrumbData(t *testing.T) {
	h := hubFixture(t)
	defer h.Close()

	w := htmlGet(t, h.Handler(), "/p/alpha/", false)
	if w.Code != http.StatusOK {
		t.Fatalf("page = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `data-base="/p/alpha"`) {
		t.Error("project page missing data-base mount attribute")
	}
	if !strings.Contains(body, `data-project-name="`) {
		t.Error("project page missing data-project-name attribute")
	}
	if strings.Contains(body, `id="project-menu"`) || strings.Contains(body, "project-nav") {
		t.Error("project dropdown markup must not render")
	}

	// The landing has no location nav at all (it is the trail's root).
	w = htmlGet(t, h.Handler(), "/", false)
	if strings.Contains(w.Body.String(), `id="location-nav"`) {
		t.Error("landing should render no breadcrumb nav")
	}
}

func TestLegacyModeKeepsRootTrail(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	defer s.Close()
	w := htmlGet(t, s.Handler(), "/", false)
	body := w.Body.String()
	if !strings.Contains(body, `data-base=""`) || !strings.Contains(body, `data-project-name="`) {
		t.Error("legacy page missing base/project-name attributes")
	}
	if strings.Contains(body, `id="project-menu"`) {
		t.Error("legacy mode rendered project dropdown markup")
	}
	// The static location current label still reads "Changes" pre-JS.
	if !strings.Contains(body, `id="location-current">Changes<`) {
		t.Error("legacy location nav missing Changes root label")
	}
}

func TestHubSelfHealsRegistryDrift(t *testing.T) {
	a := fixtureRepo(t)
	reg := registry.OpenStore(filepath.Join(t.TempDir(), "config.json"))
	if err := registry.Save(reg.Path(), registry.Config{Projects: []registry.Project{
		{Slug: "alpha", Path: a, Added: "2026-09-24"},
	}}); err != nil {
		t.Fatal(err)
	}
	h := NewHub(reg, hubBoot(t), "127.0.0.1", 9099)
	defer h.Close()

	// Out-of-band registration — another hub instance sharing the global
	// registry, or a hand edit: the file gains an entry, this process has
	// no slot for it.
	b := fixtureRepo(t)
	drifted := registry.Load(reg.Path())
	drifted.Projects = append(drifted.Projects, registry.Project{Slug: "beta", Path: b, Added: "2026-09-24"})
	if err := registry.Save(reg.Path(), drifted); err != nil {
		t.Fatal(err)
	}

	// The next landing/API look mounts it — no restart, no Add call.
	w := do(t, h.Handler(), "GET", "/api/projects", "")
	var resp struct{ Projects []projectRow }
	json.Unmarshal(w.Body.Bytes(), &resp)
	statuses := map[string]string{}
	for _, p := range resp.Projects {
		statuses[p.Slug] = p.Status
	}
	if statuses["beta"] != StatusReady || statuses["alpha"] != StatusReady {
		t.Fatalf("statuses = %v, want both ready", statuses)
	}
	if h.slot("beta") == nil {
		t.Fatal("beta not mounted after self-heal")
	}
	// The mount is live and stable across looks.
	if w = do(t, h.Handler(), "GET", "/p/beta/", ""); w.Code != http.StatusOK {
		t.Fatalf("/p/beta/ = %d, want the booted project", w.Code)
	}

	// Out-of-band removal drops the slot too.
	drifted = registry.Load(reg.Path())
	drifted.Projects = drifted.Projects[:1]
	if err := registry.Save(reg.Path(), drifted); err != nil {
		t.Fatal(err)
	}
	w = do(t, h.Handler(), "GET", "/api/projects", "")
	if strings.Contains(w.Body.String(), `"beta"`) {
		t.Fatalf("removed entry still listed: %s", w.Body)
	}
	if h.slot("beta") != nil {
		t.Fatal("orphaned slot not unmounted")
	}
	if w = do(t, h.Handler(), "GET", "/p/beta/", ""); w.Code != http.StatusNotFound {
		t.Fatalf("/p/beta/ = %d, want 404 after heal", w.Code)
	}
}

func TestHubRetriesUnavailableSlots(t *testing.T) {
	a := fixtureRepo(t)
	reg := registry.OpenStore(filepath.Join(t.TempDir(), "config.json"))
	if err := registry.Save(reg.Path(), registry.Config{Projects: []registry.Project{
		{Slug: "alpha", Path: a, Added: "2026-09-24"},
	}}); err != nil {
		t.Fatal(err)
	}

	// A boot that fails once (transient cause — e.g. a not-yet-repaired
	// legacy repo) and succeeds afterwards.
	var fail atomic.Bool
	fail.Store(true)
	inner := hubBoot(t)
	boot := func(dir, basePath, publicBase string) (http.Handler, func(), error) {
		if fail.Load() {
			return nil, nil, errors.New("transient boot failure")
		}
		return inner(dir, basePath, publicBase)
	}
	h := NewHub(reg, boot, "127.0.0.1", 9099)
	defer h.Close()
	if s := h.slot("alpha"); s == nil || s.status != StatusUnavailable {
		t.Fatalf("initial slot = %+v, want unavailable", s)
	}

	// Cause fixed: the next landing/API look retries the boot and heals.
	fail.Store(false)
	w := do(t, h.Handler(), "GET", "/api/projects", "")
	if !strings.Contains(w.Body.String(), `"status":"ready"`) {
		t.Fatalf("statuses after heal = %s", w.Body)
	}
	if s := h.slot("alpha"); s == nil || s.status != StatusReady {
		t.Fatalf("healed slot = %+v, want ready", s)
	}
}
