package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"lessmess/internal/store"
)

// realBoot opens a true store + server, mirroring main's boot closure.
func realBoot(t *testing.T) func(string) (http.Handler, error) {
	t.Helper()
	return func(dir string) (http.Handler, error) {
		st, err := store.Open(dir)
		if err != nil {
			return nil, err
		}
		t.Cleanup(st.Close)
		return New(st).Handler(), nil
	}
}

func TestBootstrapFullLoop(t *testing.T) {
	dir := t.TempDir()
	s := NewSetup(dir, "", realBoot(t))

	w := do(t, s, "POST", "/api/setup/bootstrap", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body)
	}
	var resp bootstrapResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !resp.Reloaded {
		t.Fatal("reloaded = false, want hot-open after bootstrap")
	}
	byPath := map[string]string{}
	for _, a := range resp.Actions {
		byPath[a.Path] = a.Action
	}
	for _, p := range []string{"AGENTS.md", ".lessmess/workflow/index.json", ".gitignore", "opencode.json"} {
		if byPath[p] != "created" {
			t.Errorf("%s: %q, want created", p, byPath[p])
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "agentsdocs.json")); !os.IsNotExist(err) {
		t.Errorf("bootstrap created agentsdocs.json (err=%v)", err)
	}

	// Onboarding step recorded.
	st := loadOnboarding(dir)
	if st.Steps["bootstrap"] != "done" {
		t.Errorf("steps = %+v", st.Steps)
	}
	if _, ok := st.Steps["docs-coverage"]; ok {
		t.Errorf("obsolete docs-coverage step recorded: %+v", st.Steps)
	}

	// The handler swapped: the full UI answers now, no restart.
	w = do(t, s, "GET", "/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET / after swap = %d", w.Code)
	}
	var idx struct {
		Changes []map[string]any `json:"changes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &idx); err != nil {
		t.Fatalf("after swap / is not the index JSON: %v (%s)", err, w.Body)
	}

	// Re-bootstrap through the swapped (normal) server: idempotent, no reload.
	w = do(t, s, "POST", "/api/setup/bootstrap", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("re-bootstrap = %d", w.Code)
	}
	resp = bootstrapResponse{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.Reloaded {
		t.Error("reloaded = true on the normal server, want false")
	}
	for _, a := range resp.Actions {
		if a.Action != "skipped" {
			t.Errorf("re-run %s = %q, want skipped", a.Path, a.Action)
		}
	}
}

func TestBootstrapPartialTree(t *testing.T) {
	// changes/ exists but has no root ledger (user-reported case): setup
	// mode applies, and bootstrap fills in the missing ledger via init.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewSetup(dir, "", realBoot(t))

	w := do(t, s, "POST", "/api/setup/bootstrap", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body)
	}
	var resp bootstrapResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !resp.Reloaded {
		t.Fatal("reloaded = false on a partial tree, want hot-open")
	}
	w = do(t, s, "GET", "/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET / after swap = %d", w.Code)
	}
}

func TestSetupDirsListing(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"src", "dist", "docs", "changes", ".git"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "afile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSetup(dir, "", nil)

	w := do(t, s, "GET", "/api/setup/dirs", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var resp setupDirsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.Dir != "" {
		t.Errorf("dir = %q, want root", resp.Dir)
	}
	got := map[string]setupDirEntry{}
	for _, d := range resp.Dirs {
		got[d.Name] = d
	}
	if len(got) != 3 {
		t.Fatalf("dirs = %+v, want docs, dist, src (changes/.git/files skipped)", got)
	}
	if got["src"].Rel != "src" || got["src"].DefaultExcluded || got["src"].Excluded {
		t.Errorf("src = %+v", got["src"])
	}
	if !got["dist"].DefaultExcluded {
		t.Errorf("dist = %+v, want defaultExcluded", got["dist"])
	}
	if resp.HasConfig {
		t.Error("hasConfig = true without agentsdocs.json")
	}
}

func TestSetupDirsNestedAndExcluded(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"a/b/c", "a/b2", "swagger"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "agentsdocs.json"), []byte(`{"include":["**"],"exclude":["swagger","a/b"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSetup(dir, "", nil)

	// Root: swagger shows excluded (base-name pattern).
	var root setupDirsResponse
	w := do(t, s, "GET", "/api/setup/dirs", "")
	if err := json.Unmarshal(w.Body.Bytes(), &root); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !root.HasConfig {
		t.Fatal("hasConfig = false with a config present")
	}
	byName := map[string]setupDirEntry{}
	for _, d := range root.Dirs {
		byName[d.Name] = d
	}
	if !byName["swagger"].Excluded {
		t.Errorf("swagger = %+v, want excluded", byName["swagger"])
	}
	if !byName["a"].HasChildren {
		t.Errorf("a = %+v, want hasChildren", byName["a"])
	}

	// Children of a: b shows excluded (path pattern a/b), b2 does not.
	var nested setupDirsResponse
	w = do(t, s, "GET", "/api/setup/dirs?dir=a", "")
	if err := json.Unmarshal(w.Body.Bytes(), &nested); err != nil {
		t.Fatalf("json: %v", err)
	}
	if nested.Dir != "a" || len(nested.Dirs) != 2 {
		t.Fatalf("nested = %+v, want dir a with b, b2", nested)
	}
	byName = map[string]setupDirEntry{}
	for _, d := range nested.Dirs {
		byName[d.Name] = d
	}
	if !byName["b"].Excluded || byName["b"].Rel != "a/b" || !byName["b"].HasChildren {
		t.Errorf("b = %+v, want excluded a/b with children", byName["b"])
	}
	if byName["b2"].Excluded {
		t.Errorf("b2 = %+v, want not excluded", byName["b2"])
	}

	// Deeper: ?dir=a/b lists c.
	w = do(t, s, "GET", "/api/setup/dirs?dir=a/b", "")
	nested = setupDirsResponse{}
	if err := json.Unmarshal(w.Body.Bytes(), &nested); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(nested.Dirs) != 1 || nested.Dirs[0].Rel != "a/b/c" {
		t.Errorf("a/b = %+v, want [a/b/c]", nested.Dirs)
	}

	// Bad dirs are rejected; missing dirs 404.
	for _, bad := range []string{"../x", "changes", ".hidden", "a//b"} {
		if w := do(t, s, "GET", "/api/setup/dirs?dir="+bad, ""); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("dir=%q = %d, want 422", bad, w.Code)
		}
	}
	if w := do(t, s, "GET", "/api/setup/dirs?dir=nope", ""); w.Code != http.StatusNotFound {
		t.Errorf("missing dir = %d, want 404", w.Code)
	}
}

func TestBootstrapBadBody(t *testing.T) {
	s, _, _ := setupShell(t, nil)
	if w := do(t, s, "POST", "/api/setup/bootstrap", `{oops`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", w.Code)
	}
}
