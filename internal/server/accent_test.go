package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/store"
)

func TestAccentPaletteShape(t *testing.T) {
	if DefaultAccent().ID != AccentPalette[0].ID {
		t.Errorf("default = %q, want the first palette entry", DefaultAccent().ID)
	}
	seen := map[string]bool{}
	for _, a := range AccentPalette {
		if a.ID == "" || seen[a.ID] {
			t.Errorf("palette id %q empty or duplicated", a.ID)
		}
		seen[a.ID] = true
		for _, field := range []struct{ name, hex string }{
			{"dark", a.Dark}, {"darkHover", a.DarkHover}, {"light", a.Light}, {"lightHover", a.LightHover},
		} {
			if len(field.hex) != 7 || !strings.HasPrefix(field.hex, "#") {
				t.Errorf("%s: %s = %q, want #rrggbb", a.ID, field.name, field.hex)
			}
		}
		got, ok := AccentByID(a.ID)
		if !ok || got != a {
			t.Errorf("AccentByID(%q) = %+v,%v want %+v,true", a.ID, got, ok, a)
		}
	}
	if _, ok := AccentByID("nope"); ok {
		t.Error("AccentByID(nope) must miss")
	}
	if _, ok := AccentByID(""); ok {
		t.Error("AccentByID(\"\") must miss")
	}
}

func TestResolveAccentRollsAndPersists(t *testing.T) {
	dir := t.TempDir()
	a := ResolveAccent(dir)
	if _, ok := AccentByID(a.ID); !ok {
		t.Fatalf("rolled accent %q is not in the palette", a.ID)
	}
	// The roll is persisted to the personal layer; the next resolve reads
	// it back rather than rolling again.
	pers, err := readSettingsLayer(settingsPersonalPath(dir))
	if err != nil {
		t.Fatalf("read personal layer: %v", err)
	}
	if pers.UI.Accent != a.ID {
		t.Fatalf("personal ui.accent = %q, want rolled %q", pers.UI.Accent, a.ID)
	}
	for range 5 {
		if again := ResolveAccent(dir); again.ID != a.ID {
			t.Fatalf("re-resolve = %q, want stable %q", again.ID, a.ID)
		}
	}
}

func TestResolveAccentRespectsConfigured(t *testing.T) {
	dir := t.TempDir()
	writeJSONFile(t, settingsProjectPath(dir), Settings{UI: UISettings{Accent: "teal"}})
	if a := ResolveAccent(dir); a.ID != "teal" {
		t.Errorf("project accent = %q, want teal", a.ID)
	}
	// Project-only rolls must not touch the personal layer.
	if _, err := os.Stat(settingsPersonalPath(dir)); !os.IsNotExist(err) {
		t.Errorf("personal layer stat err = %v, want not exist", err)
	}

	writeJSONFile(t, settingsPersonalPath(dir), Settings{UI: UISettings{Accent: "violet"}})
	if a := ResolveAccent(dir); a.ID != "violet" {
		t.Errorf("personal accent = %q, want violet (personal wins)", a.ID)
	}
}

func TestResolveAccentInvalidFailsOpen(t *testing.T) {
	dir := t.TempDir()
	writeJSONFile(t, settingsPersonalPath(dir), Settings{UI: UISettings{Accent: "chartreuse"}})
	if a := ResolveAccent(dir); a != DefaultAccent() {
		t.Errorf("invalid stored accent = %+v, want default", a)
	}
	// The bogus value is never silently rewritten.
	pers, err := readSettingsLayer(settingsPersonalPath(dir))
	if err != nil {
		t.Fatalf("read personal layer: %v", err)
	}
	if pers.UI.Accent != "chartreuse" {
		t.Errorf("personal ui.accent = %q, want untouched", pers.UI.Accent)
	}
}

func TestResolveAccentRollPreservesPersonalUI(t *testing.T) {
	dir := t.TempDir()
	writeJSONFile(t, settingsPersonalPath(dir), Settings{UI: UISettings{ShowArchived: boolp(false)}})
	a := ResolveAccent(dir)
	pers, err := readSettingsLayer(settingsPersonalPath(dir))
	if err != nil {
		t.Fatalf("read personal layer: %v", err)
	}
	if pers.UI.Accent != a.ID {
		t.Errorf("personal ui.accent = %q, want %q", pers.UI.Accent, a.ID)
	}
	if pers.UI.ShowArchived == nil || *pers.UI.ShowArchived {
		t.Errorf("personal ui.showArchived = %v, want false preserved", pers.UI.ShowArchived)
	}
}

func TestEffectiveAccentColorDoesNotRoll(t *testing.T) {
	dir := t.TempDir()
	if a := effectiveAccentColor(dir); a != DefaultAccent() {
		t.Errorf("unset = %+v, want default", a)
	}
	if _, err := os.Stat(settingsPersonalPath(dir)); !os.IsNotExist(err) {
		t.Errorf("effectiveAccentColor must not persist: stat err = %v", err)
	}
	writeJSONFile(t, settingsProjectPath(dir), Settings{UI: UISettings{Accent: "pink"}})
	if a := effectiveAccentColor(dir); a.ID != "pink" {
		t.Errorf("configured = %q, want pink", a.ID)
	}
}

func TestSettingsAPIPutValidatesAccent(t *testing.T) {
	s := mappingServer(t, nil) // nil oc: accent validation must still run

	w := do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"ui":{"accent":"chartreuse"}}`)
	if w.Code != 422 {
		t.Errorf("unknown accent code = %d, want 422 (%s)", w.Code, w.Body)
	}
	if _, err := os.Stat(settingsPersonalPath(s.st.Dir)); !os.IsNotExist(err) {
		t.Errorf("rejected PUT must not write: stat err = %v", err)
	}

	w = do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"ui":{"accent":"cyan"}}`)
	if w.Code != 200 {
		t.Fatalf("valid accent code = %d (%s)", w.Code, w.Body)
	}
	pers, err := readSettingsLayer(settingsPersonalPath(s.st.Dir))
	if err != nil {
		t.Fatalf("read personal layer: %v", err)
	}
	if pers.UI.Accent != "cyan" {
		t.Errorf("stored accent = %q, want cyan", pers.UI.Accent)
	}
}

func TestSettingsOptionsExposeAccents(t *testing.T) {
	s := mappingServer(t, nil) // offline: the palette must still be listed
	w := do(t, s.Handler(), "GET", "/api/settings/options", "")
	if w.Code != 200 {
		t.Fatalf("options code = %d", w.Code)
	}
	var resp settingsOptionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Accents) != len(AccentPalette) {
		t.Fatalf("accents = %d entries, want %d", len(resp.Accents), len(AccentPalette))
	}
	if resp.Accents[0].ID != "orange" || resp.Accents[0].Hex != DefaultAccent().Dark {
		t.Errorf("first accent = %+v, want orange with its dark hex", resp.Accents[0])
	}
	// The client live-previews picks from these values, so all four must
	// be present for every entry.
	for _, a := range resp.Accents {
		if a.Hex == "" || a.DarkHover == "" || a.Light == "" || a.LightHover == "" {
			t.Errorf("accent %q missing color values: %+v", a.ID, a)
		}
	}
}

func TestSettingsChangeAllowlistAccent(t *testing.T) {
	eff, _, _ := loadEffectiveSettings(t.TempDir())
	eff.UI.Accent = "teal"
	v, ok := settingsFieldValue(eff, "ui.accent")
	if !ok || v != "teal" {
		t.Errorf("settingsFieldValue(ui.accent) = %q,%v want teal,true", v, ok)
	}
	if _, ok := settingsFieldValue(eff, "ui.nope"); ok {
		t.Error("unknown field must miss")
	}
}

func TestPersistAccentWritesAtomicFile(t *testing.T) {
	dir := t.TempDir()
	a, _ := AccentByID("blue") // known-good
	if err := persistAccent(dir, a); err != nil {
		t.Fatalf("persistAccent: %v", err)
	}
	b, err := os.ReadFile(settingsPersonalPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Errorf("persisted file is not valid JSON: %s", b)
	}
	// Personal settings live under the state dir, gitignored by init.
	if rel, err := filepath.Rel(dir, settingsPersonalPath(dir)); err != nil || !strings.HasPrefix(rel, store.StateDirName+"/") {
		t.Errorf("personal accent path %q outside state dir (rel %q, err %v)", settingsPersonalPath(dir), rel, err)
	}
}
