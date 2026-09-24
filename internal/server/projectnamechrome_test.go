package server

import (
	"path/filepath"
	"strings"
	"testing"
)

// The project name renders server-side in the header brand and tab <title>.
// The renderer hook fills pageData centrally, so every page (and setup) shows it.

func writeProjectName(t *testing.T, dir, name string) {
	t.Helper()
	writeJSONFile(t, settingsProjectPath(dir), Settings{
		General: GeneralSettings{ProjectName: name},
	})
}

func TestProjectNameInChrome(t *testing.T) {
	st, dir := fixtureStore(t)
	writeProjectName(t, dir, "Atlas")
	h := New(st).Handler()

	for _, path := range []string{"/", "/settings", "/explorer", "/setup"} {
		w := htmlGet(t, h, path, false)
		if w.Code != 200 {
			t.Fatalf("%s: code = %d", path, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "<title>Atlas</title>") {
			t.Errorf("%s: tab title missing the project name", path)
		}
		if got := strings.Count(body, `<span class="brand-name">Atlas</span>`); got != 1 {
			t.Errorf("%s: brand-name spans = %d, want 1", path, got)
		}
	}
}

func TestProjectNameChromeFallsBackToDirectory(t *testing.T) {
	st, dir := fixtureStore(t)
	h := New(st).Handler()

	want := filepath.Base(dir)
	w := htmlGet(t, h, "/", false)
	body := w.Body.String()
	if !strings.Contains(body, "<title>"+want+"</title>") {
		t.Errorf("tab title = unset name, want fallback %q", want)
	}
	if got := strings.Count(body, `<span class="brand-name">`+want+`</span>`); got != 1 {
		t.Errorf("brand-name spans = %d, want 1 (fallback name)", got)
	}
}

func TestProjectNameHTMLScapedInChrome(t *testing.T) {
	st, dir := fixtureStore(t)
	writeProjectName(t, dir, `<b>Alt</b>`)
	h := New(st).Handler()

	w := htmlGet(t, h, "/", false)
	body := w.Body.String()
	if strings.Contains(body, "<title><b>Alt</b></title>") {
		t.Error("project name must be HTML-escaped in the title")
	}
	if !strings.Contains(body, "<title>&lt;b&gt;Alt&lt;/b&gt;</title>") {
		t.Error("escaped project name missing from the title")
	}
}
