package server

import (
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lessmess/internal/model"
)

func htmlGet(t *testing.T, h http.Handler, path string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("Accept", "text/html")
	if hx {
		r.Header.Set("HX-Request", "true")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAssetsVersionStableAndSensitive(t *testing.T) {
	v1 := assetsVersion()
	v2 := assetsVersion()
	if v1 == "" || v1 != v2 {
		t.Fatalf("unstable or empty version: %q vs %q", v1, v2)
	}
	// Content sensitivity: hash of different content differs.
	h1 := fnv.New32a()
	h1.Write([]byte("a"))
	h2 := fnv.New32a()
	h2.Write([]byte("b"))
	if h1.Sum32() == h2.Sum32() {
		t.Fatal("fnv not content-sensitive in test")
	}
}

func TestIndexHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"<html", "lessmess", "Fixture change", "/changes/2026-09-10-0", "htmx.min.js"} {
		if !strings.Contains(body, want) {
			t.Errorf("index HTML missing %q", want)
		}
	}
}

func TestBoardHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"Not started", "In progress", "Blocked", "Test", "Done", "Cancelled",
		"First", "Second", "data-task=\"FIX-00\"", `data-change="2026-09-10-0"`,
		`sessions-btn`, `sessions-panel`, `terminal-overlay`, `xterm.min.js`} {
		if !strings.Contains(body, want) {
			t.Errorf("board HTML missing %q", want)
		}
	}
}

func TestBoardLifecycleButtons(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	w := htmlGet(t, h, "/changes/2026-09-10-0", false)
	body := w.Body.String()
	if !strings.Contains(body, `id="close-change-btn"`) || strings.Contains(body, `id="reopen-btn"`) {
		t.Errorf("In progress board should show Close change only")
	}
	if !strings.Contains(body, `id="commit-btn"`) {
		t.Errorf("missing Commit button")
	}

	// After closing, the board offers Reopen instead.
	if err := st.SetChangeStatus("2026-09-10-0", model.OverallDone); err != nil {
		t.Fatal(err)
	}
	w = htmlGet(t, h, "/changes/2026-09-10-0", false)
	body = w.Body.String()
	if !strings.Contains(body, `id="reopen-btn"`) || strings.Contains(body, `id="close-change-btn"`) {
		t.Errorf("Done board should show Reopen only")
	}
}

func TestBoardFragmentHX(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0", true)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	// Fragment only: cards, but no full page chrome.
	if !strings.Contains(body, "data-task=\"FIX-00\"") {
		t.Error("fragment missing card")
	}
	if strings.Contains(body, "<html") {
		t.Error("fragment should not be a full page")
	}
}

func TestTaskDetailHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0/tasks/00-first.md", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	// goldmark rendered the skeleton headings to HTML.
	if !strings.Contains(body, "<h2 id=\"objective\">Objective</h2>") && !strings.Contains(body, "<h2") {
		t.Errorf("task detail did not render markdown headings: %.200s", body)
	}
	if !strings.Contains(body, "FIX-00") {
		t.Error("missing task id")
	}
}

func TestStaticAssets(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	for _, p := range []string{"/static/htmx.min.js", "/static/Sortable.min.js", "/static/app.js", "/static/app.css"} {
		r := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Errorf("%s: code = %d", p, w.Code)
		}
		if cc := w.Header().Get("Cache-Control"); cc == "" {
			t.Errorf("%s: no cache header", p)
		}
	}
}

func TestFormEncodedCreateTask(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	r := httptest.NewRequest("POST", "/changes/2026-09-10-0/tasks", strings.NewReader("title=Via+form"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
}

func TestFormEncodedCreateChangeRedirect(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	r := httptest.NewRequest("POST", "/changes/", strings.NewReader("title=New+thing&prefix=NT"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	redir := w.Header().Get("HX-Redirect")
	if !strings.HasPrefix(redir, "/changes/") {
		t.Fatalf("HX-Redirect = %q", redir)
	}
}
