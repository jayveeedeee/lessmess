package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/store"
)

// writeUserCLIConfig points XDG_CONFIG_HOME at a temp dir containing the
// given cli.json content and returns its path.
func writeUserCLIConfig(t *testing.T, content string) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if content == "" {
		return filepath.Join(xdg, "opencode", "cli.json")
	}
	path := filepath.Join(xdg, "opencode", "cli.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func generatedCLIConfig(t *testing.T, repoDir string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoDir, store.StateDirName, "xdg", "opencode", "cli.json"))
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("generated config is not strict JSON: %v\n%s", err, b)
	}
	return m
}

func subMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	sub, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("key %q is %T, want object", key, m[key])
	}
	return sub
}

func TestEnsureTUIConfigNoUserFile(t *testing.T) {
	writeUserCLIConfig(t, "")
	repo := t.TempDir()
	xdg, err := ensureTUIConfig(repo)
	if err != nil {
		t.Fatalf("ensureTUIConfig: %v", err)
	}
	if want := filepath.Join(repo, store.StateDirName, "xdg"); xdg != want {
		t.Fatalf("xdg = %q, want %q", xdg, want)
	}
	cfg := generatedCLIConfig(t, repo)
	if got := subMap(t, cfg, "tabs")["enabled"]; got != false {
		t.Errorf("tabs.enabled = %v, want false", got)
	}
	if got := subMap(t, cfg, "session")["sidebar"]; got != "hide" {
		t.Errorf("session.sidebar = %v, want hide", got)
	}
	if got := cfg["$schema"]; got != tuiConfigSchemaURL {
		t.Errorf("$schema = %v, want %q", got, tuiConfigSchemaURL)
	}
}

func TestEnsureTUIConfigMergesUserKeys(t *testing.T) {
	user := `{
  "$schema": "https://example.com/custom.json",
  "theme": {"name": "tokyonight", "mode": "dark"},
  "keybinds": {"leader": "ctrl+space"},
  "tabs": {"enabled": true, "layout": "vertical"},
  "session": {"sidebar": "auto", "scrollbar": false}
}`
	writeUserCLIConfig(t, user)
	repo := t.TempDir()
	if _, err := ensureTUIConfig(repo); err != nil {
		t.Fatalf("ensureTUIConfig: %v", err)
	}
	cfg := generatedCLIConfig(t, repo)
	if got := subMap(t, cfg, "theme")["name"]; got != "tokyonight" {
		t.Errorf("theme.name = %v, want tokyonight (user key preserved)", got)
	}
	if got := subMap(t, cfg, "keybinds")["leader"]; got != "ctrl+space" {
		t.Errorf("keybinds.leader = %v, want ctrl+space (user key preserved)", got)
	}
	tabs := subMap(t, cfg, "tabs")
	if got := tabs["enabled"]; got != false {
		t.Errorf("tabs.enabled = %v, want false (override wins)", got)
	}
	if got := tabs["layout"]; got != "vertical" {
		t.Errorf("tabs.layout = %v, want vertical (sibling key preserved)", got)
	}
	sess := subMap(t, cfg, "session")
	if got := sess["sidebar"]; got != "hide" {
		t.Errorf("session.sidebar = %v, want hide (override wins)", got)
	}
	if got := sess["scrollbar"]; got != false {
		t.Errorf("session.scrollbar = %v, want false (sibling key preserved)", got)
	}
	if got := cfg["$schema"]; got != "https://example.com/custom.json" {
		t.Errorf("$schema = %v, want user's own value preserved", got)
	}
}

func TestEnsureTUIConfigMergesJSONC(t *testing.T) {
	user := `{
  // user's preferred theme
  "theme": {"name": "gruvbox",},
  /* sidebar left on for
     standalone use */
  "session": {"sidebar": "auto"},
}`
	writeUserCLIConfig(t, user)
	repo := t.TempDir()
	if _, err := ensureTUIConfig(repo); err != nil {
		t.Fatalf("ensureTUIConfig: %v", err)
	}
	cfg := generatedCLIConfig(t, repo)
	if got := subMap(t, cfg, "theme")["name"]; got != "gruvbox" {
		t.Errorf("theme.name = %v, want gruvbox (JSONC recovered)", got)
	}
	if got := subMap(t, cfg, "session")["sidebar"]; got != "hide" {
		t.Errorf("session.sidebar = %v, want hide", got)
	}
}

func TestEnsureTUIConfigUnparseableUserFile(t *testing.T) {
	writeUserCLIConfig(t, `{{{ not json at all`)
	repo := t.TempDir()
	if _, err := ensureTUIConfig(repo); err != nil {
		t.Fatalf("ensureTUIConfig must not fail on bad user config: %v", err)
	}
	cfg := generatedCLIConfig(t, repo)
	if got := subMap(t, cfg, "tabs")["enabled"]; got != false {
		t.Errorf("tabs.enabled = %v, want false", got)
	}
	if _, dropped := cfg["theme"]; dropped {
		t.Errorf("unparseable user content must be dropped, got theme %v", cfg["theme"])
	}
}

func TestEnsureTUIConfigNonObjectOverrideParents(t *testing.T) {
	writeUserCLIConfig(t, `{"tabs": true, "session": "weird", "theme": {"name": "nord"}}`)
	repo := t.TempDir()
	if _, err := ensureTUIConfig(repo); err != nil {
		t.Fatalf("ensureTUIConfig: %v", err)
	}
	cfg := generatedCLIConfig(t, repo)
	if got := subMap(t, cfg, "tabs")["enabled"]; got != false {
		t.Errorf("tabs.enabled = %v, want false", got)
	}
	if got := subMap(t, cfg, "session")["sidebar"]; got != "hide" {
		t.Errorf("session.sidebar = %v, want hide", got)
	}
	if got := subMap(t, cfg, "theme")["name"]; got != "nord" {
		t.Errorf("theme.name = %v, want nord", got)
	}
}

func TestEnsureTUIConfigLeavesUserFileUntouched(t *testing.T) {
	user := "{\n  \"theme\": {\"name\": \"tokyonight\"},\n}\n"
	path := writeUserCLIConfig(t, user)
	repo := t.TempDir()
	if _, err := ensureTUIConfig(repo); err != nil {
		t.Fatalf("ensureTUIConfig: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != user {
		t.Errorf("user cli.json modified: got %q", b)
	}
}

func TestXDGEnvReplacesInherited(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/inherited")
	t.Setenv("TUI_KEEP_ME", "kept")
	env := xdgEnv("/managed/xdg")
	var xdgEntries, kept int
	for _, e := range env {
		if strings.HasPrefix(e, "XDG_CONFIG_HOME=") {
			xdgEntries++
			if e != "XDG_CONFIG_HOME=/managed/xdg" {
				t.Errorf("stale XDG entry survived: %q", e)
			}
		}
		if e == "TUI_KEEP_ME=kept" {
			kept++
		}
	}
	if xdgEntries != 1 {
		t.Errorf("XDG_CONFIG_HOME entries = %d, want exactly 1", xdgEntries)
	}
	if kept != 1 {
		t.Errorf("unrelated env var lost: %v", env)
	}
}

func TestStripJSONCStringsUnscathed(t *testing.T) {
	in := `{"a": "http://x/*y*/", "b": "}", "c": 1,}`
	out := stripJSONC([]byte(in))
	m, ok := parseCLIConfig(out)
	if !ok {
		t.Fatalf("stripped output does not parse: %s", out)
	}
	if m["a"] != "http://x/*y*/" || m["b"] != "}" {
		t.Errorf("string contents mangled: %v", m)
	}
	if _, ok := m["c"]; !ok {
		t.Errorf("key c lost (trailing comma strip): %s", out)
	}
	if strings.Contains(string(out), ",") && strings.HasSuffix(strings.TrimSpace(string(out)), ",}") {
		t.Errorf("trailing comma survived: %s", out)
	}
}
