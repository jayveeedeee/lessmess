package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	s := NewSetup(dir, realBoot(t))

	// Bootstrap with coverage enabled.
	w := do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true}`)
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
	for _, p := range []string{"AGENTS.md", ".lessmess/workflow/index.json", ".gitignore", "opencode.json", "agentsdocs.json"} {
		if byPath[p] != "created" {
			t.Errorf("%s: %q, want created", p, byPath[p])
		}
	}

	// Onboarding step recorded.
	st := loadOnboarding(dir)
	if st.Steps["bootstrap"] != "done" || st.Steps["docs-coverage"] != "enabled" {
		t.Errorf("steps = %+v", st.Steps)
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
	w = do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true}`)
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

func TestBootstrapSkipCoverage(t *testing.T) {
	dir := t.TempDir()
	s := NewSetup(dir, realBoot(t))

	w := do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, "agentsdocs.json")); !os.IsNotExist(err) {
		t.Errorf("agentsdocs.json exists despite docsCoverage:false (err=%v)", err)
	}
	if st := loadOnboarding(dir); st.Steps["docs-coverage"] != "disabled" {
		t.Errorf("steps = %+v, want docs-coverage disabled", st.Steps)
	}
	// Boot still fires (changes/ exists) even without coverage.
	var resp bootstrapResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !resp.Reloaded {
		t.Error("reloaded = false without coverage, want true (changes/ exists)")
	}
}

func TestBootstrapPartialTree(t *testing.T) {
	// changes/ exists but has no root ledger (user-reported case): setup
	// mode applies, and bootstrap fills in the missing ledger via init.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewSetup(dir, realBoot(t))

	w := do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":false}`)
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
	s := NewSetup(dir, nil)

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
	s := NewSetup(dir, nil)

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

func TestBootstrapExcludeDirs(t *testing.T) {
	dir := t.TempDir()
	s := NewSetup(dir, realBoot(t))
	w := do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true,"excludeDirs":["docs","swagger"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body)
	}
	data, err := os.ReadFile(filepath.Join(dir, "agentsdocs.json"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{`"docs"`, `"swagger"`} {
		if !strings.Contains(body, want) {
			t.Errorf("agentsdocs.json missing %s: %s", want, body)
		}
	}
	// Bad patterns are rejected with 422 and write nothing.
	for _, bad := range []string{"build*", "a|b", "..", ".hidden", "changes", "a//b"} {
		if w := do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true,"excludeDirs":["`+bad+`"]}`); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("pattern %q = %d, want 422", bad, w.Code)
		}
	}
}

func TestBootstrapNestedExcludeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src", "gen"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewSetup(dir, realBoot(t))

	// Select a nested path; an ancestor's redundant child is normalized away.
	w := do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true,"excludeDirs":["src","src/gen"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "agentsdocs.json"))
	if strings.Contains(string(data), "src/gen") {
		t.Errorf("redundant nested pattern must be normalized away: %s", data)
	}

	// Replace with just the nested path: src dropped, src/gen kept.
	w = do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true,"excludeDirs":["src/gen"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap 2 = %d", w.Code)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "agentsdocs.json"))
	body := string(data)
	if !strings.Contains(body, `"src/gen"`) || strings.Contains(body, `"src"`) {
		t.Errorf("want only src/gen excluded: %s", body)
	}

	// Deselect entirely: the picker-expressible path is dropped.
	w = do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true,"excludeDirs":[]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap 3 = %d", w.Code)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "agentsdocs.json"))
	if strings.Contains(string(data), "src") {
		t.Errorf("deselect must drop src/gen: %s", data)
	}
}

func TestBootstrapUpdatesExistingConfigExcludes(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"docs", "scripts", "src", "changes"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "changes", "ledger.md"), []byte(`# Changes — Root Ledger

One row per change directory. Task statuses live exclusively in each change's `+"`ledger.md`"+`.

| Change | Title | ID prefix | Branch | Status | Created | Last updated |
| --- | --- | --- | --- | --- | --- | --- |
`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Existing config: a hand-authored path pattern plus a base-name one.
	if err := os.WriteFile(filepath.Join(dir, "agentsdocs.json"), []byte(`{"include":["**"],"exclude":["web/static","docs"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSetup(dir, realBoot(t))

	// Submit a new base-name selection: "docs" is dropped (not resubmitted),
	// "web/static" is preserved, "scripts" is added.
	w := do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true,"excludeDirs":["scripts"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body)
	}
	var resp bootstrapResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	actionFor := ""
	for _, a := range resp.Actions {
		if a.Path == "agentsdocs.json" {
			actionFor = a.Action
		}
	}
	if actionFor != "merged" {
		t.Errorf("agentsdocs.json action = %q, want merged", actionFor)
	}
	data, err := os.ReadFile(filepath.Join(dir, "agentsdocs.json"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, `"web/static"`) || !strings.Contains(body, `"scripts"`) {
		t.Errorf("config must keep web/static and add scripts: %s", body)
	}
	if strings.Contains(body, `"docs"`) {
		t.Errorf("deselected base-name pattern must be dropped: %s", body)
	}

	// Same selection again: no change, action back to skipped.
	w = do(t, s, "POST", "/api/setup/bootstrap", `{"docsCoverage":true,"excludeDirs":["scripts"]}`)
	resp = bootstrapResponse{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	for _, a := range resp.Actions {
		if a.Path == "agentsdocs.json" && a.Action != "skipped" {
			t.Errorf("second identical submission: action = %q, want skipped", a.Action)
		}
	}
}

func TestBootstrapBadBody(t *testing.T) {
	s, _, _ := setupShell(t, nil)
	if w := do(t, s, "POST", "/api/setup/bootstrap", `{oops`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", w.Code)
	}
}

func TestBootstrapDefaultCoverageOn(t *testing.T) {
	dir := t.TempDir()
	s := NewSetup(dir, realBoot(t))
	w := do(t, s, "POST", "/api/setup/bootstrap", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "agentsdocs.json")); err != nil {
		t.Errorf("empty body must default coverage on: %v", err)
	}
}
