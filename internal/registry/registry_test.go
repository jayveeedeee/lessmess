package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	cfg := Load(filepath.Join(t.TempDir(), "absent.json"))
	if len(cfg.Projects) != 0 {
		t.Fatalf("expected empty registry, got %d projects", len(cfg.Projects))
	}
}

func TestLoadMalformedFailsOpen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Load(p)
	if len(cfg.Projects) != 0 {
		t.Fatalf("expected empty registry for malformed file, got %d projects", len(cfg.Projects))
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg := Config{Projects: []Project{{Slug: "demo", Path: "/tmp/demo", Added: "2026-09-24"}}}
	if err := Save(p, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := Load(p)
	if len(got.Projects) != 1 || got.Projects[0].Slug != "demo" || got.Projects[0].Path != "/tmp/demo" {
		t.Fatalf("roundtrip mismatch: %+v", got.Projects)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"tasktracker", "tasktracker"},
		{"My Cool Project", "my-cool-project"},
		{"  Spaced__Name!!  ", "spaced-name"},
		{"acme---core", "acme-core"},
		{"2026_release", "2026-release"},
		{"!!!", "project"},
		{"", "project"},
		{"ümlaut Ünïcode", "mlaut-n-code"}, // non-ASCII letters are not URL-safe; dropped
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	long := Slugify(strings.Repeat("x", 100))
	if len(long) > maxSlugLen {
		t.Errorf("Slugify length %d exceeds cap %d", len(long), maxSlugLen)
	}
}

func TestAddValidates(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		dir  string
		want error
	}{
		{"relative", "rel/path", ErrInvalidPath},
		{"missing", filepath.Join(root, "absent"), ErrMissing},
		{"file", file, ErrNotDir},
	}
	for _, c := range cases {
		var cfg Config
		if _, err := Add(&cfg, c.dir); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestAddSlugDedupeAndDuplicatePath(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "demo")
	b := filepath.Join(root, "demo2")
	for _, d := range []string{a, b} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var cfg Config
	p1, err := Add(&cfg, a)
	if err != nil {
		t.Fatalf("add a: %v", err)
	}
	if p1.Slug != "demo" {
		t.Fatalf("first slug = %q, want demo", p1.Slug)
	}
	p2, err := Add(&cfg, b)
	if err != nil {
		t.Fatalf("add b: %v", err)
	}
	if p2.Slug != "demo2" {
		t.Fatalf("second slug = %q, want demo2", p2.Slug)
	}
	// Same path twice → ErrDuplicate (cleaned comparison).
	if _, err := Add(&cfg, a + "/./"); !errors.Is(err, ErrDuplicate) {
		t.Errorf("re-add got %v, want ErrDuplicate", err)
	}
	// A second directory with the same basename gets a suffixed slug.
	c := filepath.Join(root, "nested", "demo")
	if err := os.MkdirAll(c, 0o755); err != nil {
		t.Fatal(err)
	}
	p3, err := Add(&cfg, c)
	if err != nil {
		t.Fatalf("add c: %v", err)
	}
	if p3.Slug != "demo-2" {
		t.Fatalf("colliding slug = %q, want demo-2", p3.Slug)
	}
}

func TestGetAndRemove(t *testing.T) {
	cfg := Config{Projects: []Project{{Slug: "demo", Path: "/tmp/demo", Added: "2026-09-24"}}}
	if p, ok := Get(cfg, "demo"); !ok || p.Path != "/tmp/demo" {
		t.Fatalf("Get existing = %+v %v", p, ok)
	}
	if _, ok := Get(cfg, "nope"); ok {
		t.Fatal("Get unknown reported found")
	}
	if !Remove(&cfg, "demo") {
		t.Fatal("Remove existing reported false")
	}
	if Remove(&cfg, "demo") {
		t.Fatal("Remove unknown reported true")
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("expected empty after remove, got %+v", cfg.Projects)
	}
}

func TestStoreOperations(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "alpha")
	b := filepath.Join(root, "beta")
	for _, d := range []string{a, b} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := OpenStore(filepath.Join(root, "config.json"))
	if _, err := s.Add(a); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if _, err := s.Add(b); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if got := len(s.List().Projects); got != 2 {
		t.Fatalf("list = %d projects, want 2", got)
	}
	if _, ok := s.Get("alpha"); !ok {
		t.Fatal("Get alpha not found")
	}
	if !s.Remove("alpha") {
		t.Fatal("Remove alpha reported false")
	}
	if got := len(s.List().Projects); got != 1 {
		t.Fatalf("list after remove = %d projects, want 1", got)
	}
	if s.Remove("alpha") {
		t.Fatal("double Remove reported true")
	}
	if _, err := s.Add(root + "/absent"); !errors.Is(err, ErrMissing) {
		t.Errorf("add missing got %v, want ErrMissing", err)
	}
	// The failed add must not have persisted anything.
	if got := len(s.List().Projects); got != 1 {
		t.Fatalf("list after failed add = %d projects, want 1", got)
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv(EnvVar, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/u")
	p, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/home/u/.config/lessmess/config.json"; p != want {
		t.Fatalf("DefaultPath = %q, want %q", p, want)
	}
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if p, _ = DefaultPath(); p != "/xdg/lessmess/config.json" {
		t.Fatalf("XDG override = %q", p)
	}
	t.Setenv(EnvVar, "/env/override.json")
	if p, _ = DefaultPath(); p != "/env/override.json" {
		t.Fatalf("env override = %q", p)
	}
}
