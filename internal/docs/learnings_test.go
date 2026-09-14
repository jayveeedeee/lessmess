package docs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lessmess/internal/model"
)

func TestPathLikeRefsExtraction(t *testing.T) {
	auto := model.DocMarkerBegin + "\n" + `## Learnings

- (2026-09-10-0) see ` + "`internal/model/docfile.go`" + ` for the parser
- relative ref ` + "`docfile.go`" + ` works too
- model ids like ` + "`anthropic/claude-sonnet-4-5`" + ` are not paths
- bare identifiers ` + "`spawnSession`" + ` are ignored
- snippets with spaces ` + "`go test ./...`" + ` are ignored
- URLs like ` + "`https://example.com/x.go`" + ` are ignored
- hidden paths ` + "`.lessmess/queue.json`" + ` are skipped
- trailing punctuation ` + "`README.md`, and `app.js`:" + ` trims fine
- bare dirs ` + "`internal/`" + ` and versions ` + "`v1`" + ` are not path-shaped
` + model.DocMarkerEnd + "\n"
	got := pathLikeRefs(auto)
	want := []string{"README.md", "app.js", "docfile.go", "internal/model/docfile.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCleanRef(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"a/b.go", "a/b.go"},
		{" a/b.go ", "a/b.go"},
		{"a/b.go,", "a/b.go"},
		{"a/b.go.", "a/b.go"},
		{"settings.json", "settings.json"},
		{"x.c", "x.c"},
		{"Go test", ""},
		{"", ""},
		{"https://x/y.go", ""},
		{"http://x/y.go", ""},
		{".hidden/x.go", ""},
		{"./x.go", ""},
		{"x.go/", ""},  // trailing slash: extensionless final segment
		{"internal/", ""}, // bare dir mention
		{"provider/model-x", ""},
		{"v1", ""},
		{"x.toolongext", ""}, // unknown extension
		{"plainword", ""},
		// Go-style qualified identifiers must never lint.
		{"server.New", ""},
		{"store.Open", ""},
		{"Server.index", ""},
		{"renderer.setup", ""},
		{"pageData.Page", ""},
		{"s.oc", ""},
		{"s.docsQ", ""},
		{"*model.Error", ""},
		{"cmd/<name>/main.go", ""},
	}
	for _, tc := range cases {
		if got := cleanRef(tc.in); got != tc.want {
			t.Errorf("cleanRef(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// staleRefsRepo builds a covered tree with learnings to lint:
//
//	README.md
//	internal/model/model.go
//	internal/model/AGENTS.md   (cites existing + missing paths)
//	plain/AGENTS.md            (no marker section: skipped)
func staleRefsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"README.md", "internal/model/model.go"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := model.DocMarkerBegin + "\n## Learnings\n\n" +
		"- repo-relative `internal/model/model.go` exists\n" +
		"- dir-relative `model.go` exists\n" +
		"- root file `README.md` exists\n" +
		"- entry name `model.go` exists\n" +
		"- missing `gone/deleted.go` does not\n" +
		"- missing `also-gone.js` does not\n" +
		model.DocMarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(root, "internal", "model", AgentsFile), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	plain := "# plain\n\nHuman text citing `gone/away.go` outside the markers.\n"
	if err := os.MkdirAll(filepath.Join(root, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plain", AgentsFile), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStaleLearningRefs(t *testing.T) {
	root := staleRefsRepo(t)
	got, err := StaleLearningRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"internal/model": {"also-gone.js", "gone/deleted.go"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Healing: creating the missing file clears the finding.
	if err := os.MkdirAll(filepath.Join(root, "gone"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gone", "deleted.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = StaleLearningRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	want["internal/model"] = []string{"also-gone.js"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after healing: got %v, want %v", got, want)
	}
}

func TestStaleLearningRefsDisabled(t *testing.T) {
	got, err := StaleLearningRefs(t.TempDir())
	if err != nil || got != nil {
		t.Errorf("disabled repo: got %v, %v; want nil, nil", got, err)
	}
}

// A reference whose sibling variant exists (truservice.yaml.template next
// to a cited truservice.yaml) is a generated artifact: resolved, not
// flagged — even when the cursor-… er, even when the file itself is
// never committed.
func TestStaleLearningRefsSiblingVariant(t *testing.T) {
	root := staleRefsRepo(t)
	if err := os.WriteFile(filepath.Join(root, "internal", "model", "truservice.yaml.template"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "internal", "model", AgentsFile))
	if err != nil {
		t.Fatal(err)
	}
	agents := strings.Replace(string(data), "## Learnings\n", "## Learnings\n\n- renders into `truservice.yaml` at deploy time\n", 1)
	if err := os.WriteFile(filepath.Join(root, "internal", "model", AgentsFile), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := StaleLearningRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range got["internal/model"] {
		if ref == "truservice.yaml" {
			t.Errorf("sibling variant must resolve: %v", got["internal/model"])
		}
	}
}

func TestStaleLearningRefsResolvesStateFiles(t *testing.T) {
	root := staleRefsRepo(t)
	// .lessmess state resolves by bare name and by prefixed path.
	if err := os.MkdirAll(filepath.Join(root, ".lessmess", "xdg", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lessmess", "sessions.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lessmess", "xdg", "opencode", "cli.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := model.DocMarkerBegin + "\n- state in `sessions.json` and `xdg/opencode/cli.json`\n- child file `settings.go`-style bare names resolve too\n" + model.DocMarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(root, "internal", "model", AgentsFile), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	// `settings.go` does not exist anywhere in this fixture, so it alone is
	// flagged; the state references resolve.
	got, err := StaleLearningRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"internal/model": {"settings.go"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestValidateDocsLintFinding(t *testing.T) {
	root := staleRefsRepo(t)
	var found bool
	for _, f := range ValidateDocs(root, nil) {
		if f.Severity == SeverityWarning && f.File == "internal/model/AGENTS.md" &&
			strings.Contains(f.Msg, `learning cites missing path "gone/deleted.go"`) {
			found = true
		}
		if f.Severity == SeverityError {
			t.Errorf("lint must only warn, got error finding: %v", f)
		}
	}
	if !found {
		t.Error("lint finding not reported by ValidateDocs")
	}
}

// Identical missing paths across many directories collapse into one
// finding, so a repo-wide convention cannot bury the bell.
func TestValidateDocsLintFindingDedup(t *testing.T) {
	root := staleRefsRepo(t)
	cite := func(rel string) error {
		agents := model.DocMarkerBegin + "\n- config lives in `shared.yaml`, generated at deploy\n" + model.DocMarkerEnd + "\n"
		p := filepath.Join(root, filepath.FromSlash(rel), AgentsFile)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(agents), 0o644)
	}
	for _, rel := range []string{"cmd", "web"} {
		if err := cite(rel); err != nil {
			t.Fatal(err)
		}
	}
	var hits []Finding
	for _, f := range ValidateDocs(root, nil) {
		if strings.Contains(f.Msg, `"shared.yaml"`) {
			hits = append(hits, f)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("shared.yaml findings = %d, want 1 deduplicated finding", len(hits))
	}
	if !strings.Contains(hits[0].Msg, "2 directories") || !strings.Contains(hits[0].Msg, "e.g. cmd") {
		t.Errorf("dedup message missing count/example: %s", hits[0].Msg)
	}
	if hits[0].File != "cmd/AGENTS.md" {
		t.Errorf("dedup file = %q, want the first dir's AGENTS.md", hits[0].File)
	}
}
