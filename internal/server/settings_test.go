package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/store"
)

func boolp(b bool) *bool { return &b }

func TestSettingsDefaultsOnly(t *testing.T) {
	dir := t.TempDir()
	eff, sources, loadErr := loadEffectiveSettings(dir)
	if loadErr != "" {
		t.Fatalf("loadErr = %q, want empty", loadErr)
	}
	if !eff.Session.AutoOpenTerminal || !eff.UI.ShowArchived || !eff.Docs.AutoGardenerOnClose {
		t.Errorf("bool defaults = %v/%v/%v, want all true",
			eff.Session.AutoOpenTerminal, eff.UI.ShowArchived, eff.Docs.AutoGardenerOnClose)
	}
	if eff.Session.Agent != "" || eff.Session.Model != "" || eff.Git.DefaultBranch != "" {
		t.Errorf("string defaults = %q/%q/%q, want all empty",
			eff.Session.Agent, eff.Session.Model, eff.Git.DefaultBranch)
	}
	for field, src := range sources {
		if src != "default" {
			t.Errorf("sources[%s] = %q, want default", field, src)
		}
	}
	if len(sources) != 12 {
		t.Errorf("len(sources) = %d, want 12", len(sources))
	}
}

func TestSettingsProjectAndPersonalLayers(t *testing.T) {
	dir := t.TempDir()
	writeJSONFile(t, settingsProjectPath(dir), Settings{
		Session: SessionSettings{Agent: "build", Model: "prov/proj-model", AutoOpenTerminal: boolp(false)},
		Prompts: PromptSettings{Discussion: "from project"},
		Git:     GitSettings{DefaultBranch: "main"},
	})
	writeJSONFile(t, settingsPersonalPath(dir), Settings{
		Session: SessionSettings{Model: "me/personal-model"},
		Docs:    DocsSettings{AutoGardenerOnClose: boolp(false)},
	})

	eff, sources, loadErr := loadEffectiveSettings(dir)
	if loadErr != "" {
		t.Fatalf("loadErr = %q, want empty", loadErr)
	}

	// Personal wins where set; project otherwise; defaults where neither.
	if eff.Session.Agent != "build" || sources["session.agent"] != "project" {
		t.Errorf("agent = %q (%s), want build (project)", eff.Session.Agent, sources["session.agent"])
	}
	if eff.Session.Model != "me/personal-model" || sources["session.model"] != "personal" {
		t.Errorf("model = %q (%s), want me/personal-model (personal)", eff.Session.Model, sources["session.model"])
	}
	if eff.Session.AutoOpenTerminal != false || sources["session.autoOpenTerminal"] != "project" {
		t.Errorf("autoOpenTerminal = %v (%s), want false (project)", eff.Session.AutoOpenTerminal, sources["session.autoOpenTerminal"])
	}
	if eff.Prompts.Discussion != "from project" || sources["prompts.discussion"] != "project" {
		t.Errorf("discussion addendum = %q (%s)", eff.Prompts.Discussion, sources["prompts.discussion"])
	}
	if eff.Git.DefaultBranch != "main" || sources["git.defaultBranch"] != "project" {
		t.Errorf("defaultBranch = %q (%s)", eff.Git.DefaultBranch, sources["git.defaultBranch"])
	}
	if eff.Docs.AutoGardenerOnClose != false || sources["docs.autoGardenerOnClose"] != "personal" {
		t.Errorf("autoGardenerOnClose = %v (%s)", eff.Docs.AutoGardenerOnClose, sources["docs.autoGardenerOnClose"])
	}
	if !eff.UI.ShowArchived || sources["ui.showArchived"] != "default" {
		t.Errorf("showArchived = %v (%s), want true (default)", eff.UI.ShowArchived, sources["ui.showArchived"])
	}
}

func TestSettingsMalformedFailsOpen(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(settingsProjectPath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, settingsPersonalPath(dir), Settings{Git: GitSettings{DefaultBranch: "dev"}})

	eff, _, loadErr := loadEffectiveSettings(dir)
	if !strings.Contains(loadErr, "lessmess.json") {
		t.Errorf("loadErr = %q, want mention of lessmess.json", loadErr)
	}
	if eff.Git.DefaultBranch != "dev" {
		t.Errorf("defaultBranch = %q, want dev (healthy layer still applies)", eff.Git.DefaultBranch)
	}
	if !eff.UI.ShowArchived {
		t.Error("defaults must materialize despite a malformed project layer")
	}
}

func TestApplySettingsPatchScopesAndClear(t *testing.T) {
	dir := t.TempDir()

	// Write project layer: agent + branch.
	if err := applySettingsPatch(dir, SettingsScopeProject,
		[]byte(`{"session":{"agent":"build"},"git":{"defaultBranch":"main"}}`)); err != nil {
		t.Fatalf("project patch: %v", err)
	}
	// Personal layer: model only.
	if err := applySettingsPatch(dir, SettingsScopePersonal,
		[]byte(`{"session":{"model":"p/m"}}`)); err != nil {
		t.Fatalf("personal patch: %v", err)
	}

	eff, _, _ := loadEffectiveSettings(dir)
	if eff.Session.Agent != "build" || eff.Session.Model != "p/m" || eff.Git.DefaultBranch != "main" {
		t.Fatalf("effective = %+v", eff)
	}

	// Untouched sections survive a patch of the same layer.
	if err := applySettingsPatch(dir, SettingsScopeProject,
		[]byte(`{"prompts":{"explorer":"extra"}}`)); err != nil {
		t.Fatalf("prompts patch: %v", err)
	}
	eff, _, _ = loadEffectiveSettings(dir)
	if eff.Session.Agent != "build" || eff.Prompts.Explorer != "extra" {
		t.Fatalf("untouched section lost: %+v", eff)
	}

	// Clearing a field (empty string) restores inheritance.
	if err := applySettingsPatch(dir, SettingsScopePersonal,
		[]byte(`{"session":{"model":""}}`)); err != nil {
		t.Fatalf("clear patch: %v", err)
	}
	eff, sources, _ := loadEffectiveSettings(dir)
	if eff.Session.Model != "" || sources["session.model"] != "default" {
		t.Errorf("after clear: model = %q (%s), want empty (default)", eff.Session.Model, sources["session.model"])
	}

	// Personal layer file lives under .lessmess/, project at the root.
	if _, err := os.Stat(settingsPersonalPath(dir)); err != nil {
		t.Errorf("personal file missing: %v", err)
	}
	if _, err := os.Stat(settingsProjectPath(dir)); err != nil {
		t.Errorf("project file missing: %v", err)
	}
	// Omitempty: cleared field is gone from the personal file.
	b, _ := os.ReadFile(settingsPersonalPath(dir))
	if strings.Contains(string(b), "model") {
		t.Errorf("personal file still contains cleared field: %s", b)
	}
}

func TestApplySettingsPatchRejects(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"bad scope":       "nowhere",
		"unknown section": `{"nope":{}}`,
		"unknown field":   `{"session":{"agenta":"x"}}`,
		"malformed":       `{`,
	}
	for name, body := range cases {
		scope := SettingsScopeProject
		if name == "bad scope" {
			scope, body = "nowhere", `{}`
		}
		if err := applySettingsPatch(dir, scope, []byte(body)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestApplySettingsPatchMalformedLayerRecoverable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(settingsProjectPath(dir), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Saving must still work: the layer is replaced from an empty base.
	if err := applySettingsPatch(dir, SettingsScopeProject, []byte(`{"git":{"defaultBranch":"main"}}`)); err != nil {
		t.Fatalf("patch over malformed layer: %v", err)
	}
	eff, _, loadErr := loadEffectiveSettings(dir)
	if loadErr != "" {
		t.Errorf("loadErr = %q after repair, want empty", loadErr)
	}
	if eff.Git.DefaultBranch != "main" {
		t.Errorf("defaultBranch = %q, want main", eff.Git.DefaultBranch)
	}
}

func TestSplitModelRef(t *testing.T) {
	for _, tc := range []struct{ in, prov, id string }{
		{"fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash", "fireworks-ai", "accounts/fireworks/models/deepseek-v4p1-flash"},
		{"anthropic/claude-sonnet-4-5", "anthropic", "claude-sonnet-4-5"},
		{"noslash", "", "noslash"},
		{"", "", ""},
	} {
		prov, id := splitModelRef(tc.in)
		if prov != tc.prov || id != tc.id {
			t.Errorf("splitModelRef(%q) = (%q, %q), want (%q, %q)", tc.in, prov, id, tc.prov, tc.id)
		}
	}
}

func TestEffectivePromptAdd(t *testing.T) {
	eff := EffectiveSettings{Prompts: PromptSettings{Gardener: "g", RepoCommit: "rc"}}
	if got := eff.PromptAdd("gardener"); got != "g" {
		t.Errorf("PromptAdd(gardener) = %q", got)
	}
	if got := eff.PromptAdd("repoCommit"); got != "rc" {
		t.Errorf("PromptAdd(repoCommit) = %q", got)
	}
	if got := eff.PromptAdd("discussion"); got != "" {
		t.Errorf("PromptAdd(discussion) = %q, want empty", got)
	}
	if got := eff.PromptAdd("nope"); got != "" {
		t.Errorf("PromptAdd(nope) = %q, want empty", got)
	}
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Guard against the two layer paths colliding.
func TestSettingsPathsDistinct(t *testing.T) {
	if settingsProjectPath("/r") == settingsPersonalPath("/r") {
		t.Fatal("layer paths must differ")
	}
	if !strings.Contains(settingsPersonalPath("/r"), store.StateDirName) {
		t.Errorf("personal path %q must live under %s", settingsPersonalPath("/r"), store.StateDirName)
	}
}
