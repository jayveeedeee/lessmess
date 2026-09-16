package server

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// The general section carries the project display name: layered like every
// other setting, with the repository directory's basename as the computed
// fallback when no layer sets it.

func TestProjectNameFallsBackToDirectoryBasename(t *testing.T) {
	s := mappingServer(t, nil)
	resp := getSettingsView(t, s)

	want := filepath.Base(s.st.Dir)
	if resp.Effective.General.ProjectName != want {
		t.Errorf("effective projectName = %q, want %q", resp.Effective.General.ProjectName, want)
	}
	if resp.DefaultProjectName != want {
		t.Errorf("defaultProjectName = %q, want %q", resp.DefaultProjectName, want)
	}
	if src := resp.Sources["general.projectName"]; src != "default" {
		t.Errorf("sources[general.projectName] = %q, want default (fallback is not a stored value)", src)
	}
	if effectiveProjectName(s.st.Dir) != want {
		t.Errorf("effectiveProjectName = %q, want %q", effectiveProjectName(s.st.Dir), want)
	}
}

func TestProjectNameLayering(t *testing.T) {
	s := mappingServer(t, nil)

	// Project layer value wins over the fallback.
	w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"general":{"projectName":"Atlas"}}`)
	if w.Code != 200 {
		t.Fatalf("PUT project: %d %s", w.Code, w.Body)
	}
	resp := getSettingsView(t, s)
	if resp.Effective.General.ProjectName != "Atlas" || resp.Sources["general.projectName"] != "project" {
		t.Errorf("project layer: name = %q (%s)", resp.Effective.General.ProjectName, resp.Sources["general.projectName"])
	}

	// Personal override wins over project.
	w = do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"general":{"projectName":"Mine"}}`)
	if w.Code != 200 {
		t.Fatalf("PUT personal: %d %s", w.Code, w.Body)
	}
	resp = getSettingsView(t, s)
	if resp.Effective.General.ProjectName != "Mine" || resp.Sources["general.projectName"] != "personal" {
		t.Errorf("personal layer: name = %q (%s)", resp.Effective.General.ProjectName, resp.Sources["general.projectName"])
	}

	// Clearing personal restores project, clearing project restores the
	// directory fallback.
	w = do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"general":{"projectName":""}}`)
	if w.Code != 200 {
		t.Fatalf("PUT clear personal: %d %s", w.Code, w.Body)
	}
	w = do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"general":{"projectName":""}}`)
	if w.Code != 200 {
		t.Fatalf("PUT clear project: %d %s", w.Code, w.Body)
	}
	resp = getSettingsView(t, s)
	want := filepath.Base(s.st.Dir)
	if resp.Effective.General.ProjectName != want || resp.Sources["general.projectName"] != "default" {
		t.Errorf("after clears: name = %q (%s), want %q (default)", resp.Effective.General.ProjectName, resp.Sources["general.projectName"], want)
	}
}

func TestProjectNamePatchRejectsUnknownFields(t *testing.T) {
	s := mappingServer(t, nil)

	w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"general":{"nope":"x"}}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown general field: code = %d body = %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "general") {
		t.Errorf("error should name the section: %s", w.Body)
	}

	w = do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"bogus":{}}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown section: code = %d body = %s", w.Code, w.Body)
	}
}

func TestSettingsFieldValueProjectName(t *testing.T) {
	e := EffectiveSettings{}
	e.General.ProjectName = "Atlas"
	if v, ok := settingsFieldValue(e, "general.projectName"); !ok || v != "Atlas" {
		t.Errorf("settingsFieldValue = %q, %v; want Atlas, true", v, ok)
	}
}

func TestFallbackProjectNameGuards(t *testing.T) {
	for in, want := range map[string]string{
		"/":                        "lessmess",
		".":                        "lessmess",
		"/tmp":                     "tmp",
		"/Users/jd/GitHub/repo":    "repo",
	} {
		if got := fallbackProjectName(in); got != want {
			t.Errorf("fallbackProjectName(%q) = %q, want %q", in, got, want)
		}
	}
}
