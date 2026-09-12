package docs

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadConfigAbsent(t *testing.T) {
	c, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Error("absent config must return nil (docs system disabled)")
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      `{`,
		"unknown field": `{"include":["**"],"nope":1}`,
		"bad glob":      `{"include":["["]}`,
		"empty pattern": `{"exclude":[""]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadConfig(writeConfig(t, body)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := LoadConfig(writeConfig(t, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Include) != 1 || c.Include[0] != "**" {
		t.Errorf("default include: %v", c.Include)
	}
}

func TestCoveredDefaults(t *testing.T) {
	c, err := LoadConfig(writeConfig(t, `{"exclude":["web/static"]}`))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		".":                    true, // root always covered
		"cmd":                  true,
		"internal":             true,
		"internal/model":       true,
		"web":                  true,
		"web/static":           false, // user exclude with slash: full path
		"static":               true,  // ...does not match other base names
		".git":                 false, // hidden
		".tasktracker":         false, // hidden
		"internal/.cache":      false, // hidden segment
		"changes":              false, // canonical workflow tree
		"changes/2026-09-12-7": false,
		"node_modules":         false, // default exclude, any depth
		"web/node_modules":     false,
		"web/static/vendor":    false, // default exclude base name
		"dist":                 false,
		"target":               false,
	}
	for rel, want := range cases {
		if got := c.Covered(rel); got != want {
			t.Errorf("Covered(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestCoveredIncludeNarrows(t *testing.T) {
	c, err := LoadConfig(writeConfig(t, `{"include":["internal","internal/**","cmd"]}`))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"internal":       true,
		"internal/model": true,
		"cmd":            true,
		"web":            false,
		"web/templates":  false,
	}
	for rel, want := range cases {
		if got := c.Covered(rel); got != want {
			t.Errorf("Covered(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestCoveredDoubleStar(t *testing.T) {
	c, err := LoadConfig(writeConfig(t, `{"exclude":["**/fixtures","internal/**/testdata"]}`))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"fixtures":               false,
		"internal/fixtures":      false,
		"internal/docs/testdata": false,
		"internal/testdata":      false,
		"internal/docs":          true,
		"web/testdata":           true,
	}
	for rel, want := range cases {
		if got := c.Covered(rel); got != want {
			t.Errorf("Covered(%q) = %v, want %v", rel, got, want)
		}
	}
}

// TestRepoConfig loads this repository's own agentsdocs.json and asserts the
// intended covered set, per the DOC-01 verification step.
func TestRepoConfig(t *testing.T) {
	c, err := LoadConfig(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("repo root agentsdocs.json not found")
	}
	cases := map[string]bool{
		".":              true,
		"cmd":            true,
		"internal":       true,
		"internal/model": true,
		"web":            true,
		"web/static":     false, // vendored assets, documented by web/static/VENDOR.md
		"changes":        false,
	}
	for rel, want := range cases {
		if got := c.Covered(rel); got != want {
			t.Errorf("Covered(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if !c.Covered("anything") || c.Covered("node_modules") || c.Covered("changes") {
		t.Errorf("default config misbehaves")
	}
}

func TestCoveredOSSeparators(t *testing.T) {
	c, err := LoadConfig(writeConfig(t, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Covered(string(filepath.Join("internal", "model"))) {
		t.Error("OS-separated path not accepted")
	}
}
