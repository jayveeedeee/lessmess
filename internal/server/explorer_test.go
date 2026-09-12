package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasktracker/internal/docs"
	"tasktracker/internal/opencode"
)

// explorerServer builds a server over the fixture repo with docs enabled and
// an annotated tree:
//
//	README.md ("Project readme")
//	cmd/main.go (unannotated)
//	internal/model/model.go ("Core status types")
func explorerServer(t *testing.T) (*Server, string) {
	t.Helper()
	st, dir := fixtureStore(t)
	if err := os.WriteFile(filepath.Join(dir, docs.ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"README.md", "cmd/main.go", "internal/model/model.go"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeStructure(t, dir, "internal/model", "Parsers and serializers.", map[string]string{"model.go": "Core status types"})
	writeStructure(t, dir, ".", "The fixture repo.", map[string]string{"README.md": "Project readme"})
	s := New(st)
	t.Cleanup(s.Close)
	return s, dir
}

func writeStructure(t *testing.T, root, rel, purpose string, blurbs map[string]string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("<!-- tasktracker:begin -->\n# Structure: " + rel + "\n\n")
	b.WriteString("<!-- tasktracker-meta: refreshed=2026-09-12 source=seed tree=abc123 -->\n\n")
	b.WriteString(purpose + "\n\n## Entries\n\n| Entry | Purpose |\n| --- | --- |\n")
	for name, blurb := range blurbs {
		b.WriteString("| `" + name + "` | " + blurb + " |\n")
	}
	b.WriteString("<!-- tasktracker:end -->\n")
	p := filepath.Join(root, filepath.FromSlash(rel), docs.StructureFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildExplorerView(t *testing.T) {
	s, dir := explorerServer(t)
	tree, err := docs.Walk(dir, s.docsQ.cfg)
	if err != nil {
		t.Fatal(err)
	}
	top := buildExplorerNode(tree)
	if top.Rel != "." || top.Purpose != "The fixture repo." {
		t.Errorf("root node: %+v", top)
	}
	if len(top.Files) != 2 || top.Files[0].Blurb != "Project readme" || top.Files[1].Name != "agentsdocs.json" {
		t.Errorf("root files: %+v", top.Files)
	}
	var cmd, internal *explorerNode
	for _, d := range top.Dirs {
		switch d.Rel {
		case "cmd":
			cmd = d
		case "internal":
			internal = d
		}
	}
	if cmd == nil || internal == nil {
		t.Fatalf("children: %+v", top.Dirs)
	}
	if cmd.Purpose != "" {
		t.Errorf("unannotated dir must have empty purpose, got %q", cmd.Purpose)
	}
	if len(internal.Dirs) != 1 || internal.Dirs[0].Rel != "internal/model" {
		t.Fatalf("internal children: %+v", internal.Dirs)
	}
	model := internal.Dirs[0]
	if model.Purpose != "Parsers and serializers." {
		t.Errorf("model purpose: %q", model.Purpose)
	}
	if len(model.Files) != 1 || model.Files[0].Blurb != "Core status types" {
		t.Errorf("model files: %+v", model.Files)
	}
}

func renderTreeFragment(t *testing.T, v explorerView) string {
	t.Helper()
	r := newRenderer()
	w := httptest.NewRecorder()
	r.render(w, r.partial, "explorerTree", v)
	return w.Body.String()
}

func TestExplorerTreeFragmentRender(t *testing.T) {
	s, _ := explorerServer(t)
	html := renderTreeFragment(t, s.buildExplorerView())
	for _, want := range []string{
		"cmd/", "internal/", `data-rel="internal/model"`,
		`hx-get="/explorer/detail?dir=internal/model"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("fragment missing %q:\n%s", want, html)
		}
	}
	// The tree is pure navigation: no purposes, files, placeholders, or chat
	// buttons — those live in the detail pane.
	for _, gone := range []string{"The fixture repo.", "Parsers and serializers.", "model.go", "no description yet", "explorer-chat"} {
		if strings.Contains(html, gone) {
			t.Errorf("tree fragment must not contain %q:\n%s", gone, html)
		}
	}
}

func renderDetailFragment(t *testing.T, n *explorerNode) string {
	t.Helper()
	r := newRenderer()
	w := httptest.NewRecorder()
	r.render(w, r.partial, "explorerDetail", n)
	return w.Body.String()
}

func TestExplorerDetailFragmentRender(t *testing.T) {
	s, dir := explorerServer(t)
	tree, err := docs.Walk(dir, s.docsQ.cfg)
	if err != nil {
		t.Fatal(err)
	}
	root := buildExplorerNode(tree)
	html := renderDetailFragment(t, findExplorerNode(root, "internal/model"))
	for _, want := range []string{
		"Parsers and serializers.", "model.go", "Core status types",
		`data-dir="internal/model"`, "xdetail-rel",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("detail fragment missing %q:\n%s", want, html)
		}
	}
	// Root has an unannotated file (agentsdocs.json): blurb placeholder.
	html = renderDetailFragment(t, root)
	if !strings.Contains(html, "The fixture repo.") || !strings.Contains(html, "Project readme") {
		t.Errorf("root detail missing purpose/blurb:\n%s", html)
	}
	if !strings.Contains(html, "—") {
		t.Error("unannotated file must render a blurb placeholder")
	}
	// A dir with no STRUCTURE.md gets the purpose placeholder.
	html = renderDetailFragment(t, findExplorerNode(root, "cmd"))
	if !strings.Contains(html, "no description yet") {
		t.Error("unannotated dir must render the purpose placeholder")
	}
}

func TestExplorerDetailEndpoint(t *testing.T) {
	s, _ := explorerServer(t)
	w := do(t, s.Handler(), "GET", "/explorer/detail?dir=internal/model", "")
	if w.Code != 200 {
		t.Fatalf("detail: %d %s", w.Code, w.Body)
	}
	for _, want := range []string{"Parsers and serializers.", "Core status types"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("detail missing %q", want)
		}
	}
	w = do(t, s.Handler(), "GET", "/explorer/detail?dir=.", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "The fixture repo.") {
		t.Errorf("root detail: %d %s", w.Code, w.Body)
	}
}

func TestExplorerDisabledState(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	t.Cleanup(s.Close)
	if s.buildExplorerView().Enabled {
		t.Error("repo without agentsdocs.json must be disabled")
	}
	w := do(t, s.Handler(), "GET", "/explorer", "")
	if w.Code != 200 {
		t.Fatalf("explorer page: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "docs system is disabled") {
		t.Error("disabled page must render guidance")
	}
}

func TestFindExplorerNode(t *testing.T) {
	s, dir := explorerServer(t)
	tree, err := docs.Walk(dir, s.docsQ.cfg)
	if err != nil {
		t.Fatal(err)
	}
	root := buildExplorerNode(tree)
	if findExplorerNode(root, ".") != root {
		t.Error("root lookup failed")
	}
	if n := findExplorerNode(root, "internal/model"); n == nil || n.Purpose != "Parsers and serializers." {
		t.Errorf("nested lookup: %+v", n)
	}
	if findExplorerNode(root, "nope") != nil {
		t.Error("missing dir must yield nil")
	}
}

func TestExplorerDetailRejectsBadDirs(t *testing.T) {
	s, _ := explorerServer(t)
	for dir, want := range map[string]int{
		"web/static":   422, // not covered by the fixture config
		"nope/missing": 422, // not covered / does not exist
		"changes":      422, // never covered
	} {
		w := do(t, s.Handler(), "GET", "/explorer/detail?dir="+dir, "")
		if w.Code != want {
			t.Errorf("dir %q: code %d, want %d (%s)", dir, w.Code, want, w.Body)
		}
	}
}

func TestExplorerDetailDocsDisabled(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	t.Cleanup(s.Close)
	w := do(t, s.Handler(), "GET", "/explorer/detail?dir=.", "")
	if w.Code != 503 {
		t.Errorf("no docs config: code %d, want 503", w.Code)
	}
}

func TestExplorerPageEnabled(t *testing.T) {
	s, _ := explorerServer(t)
	w := do(t, s.Handler(), "GET", "/explorer", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `data-page="explorer"`) {
		t.Errorf("explorer page: %d", w.Code)
	}
	// Initial page: dirs-only tree plus root's detail rendered server-side.
	for _, want := range []string{"explorer-split", `id="explorer-detail"`, "cmd/", "internal/", "The fixture repo.", "Project readme"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("page missing %q", want)
		}
	}
}

// fakeOCService fakes the opencode HTTP API: session create + prompt capture.
func fakeOCService(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var prompts []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_exp","title":"t","location":{"directory":"/x"}}}`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/prompt"):
			var body struct {
				Text string `json:"text"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			prompts = append(prompts, body.Text)
			w.Write([]byte(`{"data":{}}`))
		case r.Method == "DELETE":
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(fake.Close)
	return fake, &prompts
}

func TestExplorerChatHappyPath(t *testing.T) {
	s, _ := explorerServer(t)
	fake, prompts := fakeOCService(t)
	s.SetOpencode(opencode.New(fake.URL, "pw"))

	w := do(t, s.Handler(), "POST", "/explorer/chat", `{"dir":"internal/model"}`)
	if w.Code != 201 {
		t.Fatalf("chat: %d %s", w.Code, w.Body)
	}
	var resp sessionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Session != "ses_exp" || resp.Title != "explore internal/model" {
		t.Errorf("resp: %+v", resp)
	}
	if len(*prompts) != 1 {
		t.Fatalf("prompts: %d", len(*prompts))
	}
	p := (*prompts)[0]
	for _, want := range []string{"internal/model", "Parsers and serializers.", "Core status types", "repository ROOT", "Do not modify any files"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	// Mapped to the unassigned bucket (shows up in Discussions).
	w = do(t, s.Handler(), "GET", "/api/discussions", "")
	if !strings.Contains(w.Body.String(), "ses_exp") {
		t.Errorf("session not in discussions: %s", w.Body)
	}
}

func TestExplorerChatRejectsBadDirs(t *testing.T) {
	s, _ := explorerServer(t)
	fake, _ := fakeOCService(t)
	s.SetOpencode(opencode.New(fake.URL, "pw"))
	for dir, want := range map[string]int{
		"web/static":   422, // not covered by the fixture config
		"nope/missing": 422, // not covered / does not exist
		"changes":      422, // never covered
	} {
		w := do(t, s.Handler(), "POST", "/explorer/chat", `{"dir":"`+dir+`"}`)
		if w.Code != want {
			t.Errorf("dir %q: code %d, want %d (%s)", dir, w.Code, want, w.Body)
		}
	}
}

func TestExplorerChatServiceDown(t *testing.T) {
	s, _ := explorerServer(t)
	w := do(t, s.Handler(), "POST", "/explorer/chat", `{"dir":"internal/model"}`)
	if w.Code != 503 {
		t.Errorf("no opencode client: code %d, want 503", w.Code)
	}
}

func TestExplorerPrompt(t *testing.T) {
	p := explorerPrompt("web", "(structure)", "(agents)")
	for _, want := range []string{"web", "(structure)", "(agents)", "repository ROOT"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
